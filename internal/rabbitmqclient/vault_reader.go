package rabbitmqclient

import (
	"crypto/x509"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	ctrl "sigs.k8s.io/controller-runtime"

	vault "github.com/hashicorp/vault/api"
)

const defaultAuthPath string = "auth/kubernetes"
const defaultVaultRole string = "messaging-topology-operator"

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 . SecretReader
type SecretReader interface {
	ReadSecret(path string) (*vault.Secret, error)
}

type VaultSecretReader struct {
	client *vault.Client
}

func (s VaultSecretReader) ReadSecret(path string) (*vault.Secret, error) {
	secret, err := s.client.Logical().Read(path)
	if err != nil {
		return nil, err
	}
	return secret, nil
}

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 . SecretStoreClient
type SecretStoreClient interface {
	ReadCredentials(path string) (string, string, error)
}

type VaultClient struct {
	Reader SecretReader
}

// Created - and exported from package - for testing purposes
var (
	ReadServiceAccountTokenFunc = ReadServiceAccountToken
	ReadVaultClientSecretFunc   = ReadVaultClientSecret
	LoginToVaultFunc            = LoginToVault
)

var (
	createSecretStoreClientOnce sync.Once
	SecretClient                SecretStoreClient
	SecretClientCreationError   error
)

func GetSecretStoreClient() (SecretStoreClient, error) {
	createSecretStoreClientOnce.Do(InitializeClient())
	return SecretClient, SecretClientCreationError
}

func InitializeClient() func() {
	return func() {
		// VAULT_ADDR environment variable will be the address that pod uses to communicate with Vault.
		// returns error when not set
		vaultURL := os.Getenv("VAULT_ADDR")
		if vaultURL == "" {
			SecretClientCreationError = fmt.Errorf("VAULT_ADDR environment variable not set; cannot initialize vault client")
			return
		}

		config := vault.DefaultConfig() // modify for more granular configuration

		if strings.HasPrefix(vaultURL, "https") {
			systemCertPool, err := x509.SystemCertPool()
			if err != nil {
				SecretClientCreationError = fmt.Errorf("failed to retrieve system trusted certs: %w", err)
				return
			}
			config.HttpClient.Transport.(*http.Transport).TLSClientConfig.RootCAs = systemCertPool
		}

		vaultClient, err := vault.NewClient(config)
		if err != nil {
			SecretClientCreationError = fmt.Errorf("unable to initialize Vault client: %w", err)
			return
		}

		firstLoginAttemptResultCh := make(chan error, 1)
		go renewToken(vaultClient, firstLoginAttemptResultCh)
		err = <-firstLoginAttemptResultCh
		if err != nil {
			SecretClientCreationError = fmt.Errorf("unable to login to Vault: %w", err)
			return
		}

		SecretClient = VaultClient{
			Reader: &VaultSecretReader{client: vaultClient},
		}
	}
}

func (vc VaultClient) ReadCredentials(path string) (string, string, error) {
	secret, err := vc.Reader.ReadSecret(path)
	if err != nil {
		return "", "", fmt.Errorf("unable to read Vault secret: %w", err)
	}

	if secret == nil {
		return "", "", errors.New("returned Vault secret is nil")
	}

	if secret != nil && secret.Warnings != nil && len(secret.Warnings) > 0 {
		return "", "", fmt.Errorf("warnings were returned from Vault: %v", secret.Warnings)
	}

	if secret.Data == nil {
		return "", "", errors.New("returned Vault secret has a nil Data map")
	}

	if len(secret.Data) == 0 {
		return "", "", errors.New("returned Vault secret has an empty Data map")
	}

	if secret.Data["data"] == nil {
		return "", "", fmt.Errorf("returned Vault secret has a Data map that contains no value for key 'data'. Available keys are: %v", availableKeys(secret.Data))
	}

	data, ok := secret.Data["data"].(map[string]interface{})
	if !ok {
		return "", "", fmt.Errorf("data type assertion failed for Vault secret of type: %T and value %#v read from path %s", secret.Data["data"], secret.Data["data"], path)
	}

	username, err := getValue("username", data)
	if err != nil {
		return "", "", fmt.Errorf("unable to get username from Vault secret: %w", err)
	}

	password, err := getValue("password", data)
	if err != nil {
		return "", "", fmt.Errorf("unable to get password from Vault secret: %w", err)
	}

	return username, password, nil
}

func getValue(key string, data map[string]interface{}) (string, error) {
	result, ok := data[key].(string)
	if !ok {
		return "", fmt.Errorf("expected %s to be a string but is a %T", key, data[key])
	}

	return result, nil
}

func availableKeys(m map[string]interface{}) []string {
	result := make([]string, len(m))
	i := 0
	for k := range m {
		result[i] = k
		i++
	}
	return result
}

func login(vaultClient *vault.Client) (*vault.Secret, error) {
	logger := ctrl.LoggerFrom(nil)

	jwt, err := ReadServiceAccountTokenFunc()
	if err != nil {
		return nil, fmt.Errorf("unable to read file containing service account token: %w", err)
	}

	vaultNamespace := os.Getenv("OPERATOR_VAULT_NAMESPACE")
	if vaultNamespace != "" {
		vaultClient.SetNamespace(vaultNamespace)
	}

	loginAuthPath := os.Getenv("OPERATOR_VAULT_AUTH_PATH")
	if loginAuthPath == "" {
		loginAuthPath = defaultAuthPath
	}

	role := os.Getenv("OPERATOR_VAULT_ROLE")
	if role == "" {
		role = defaultVaultRole
	}

	logger.Info("Authenticating to Vault", "vault role", role, "vault namespace", vaultNamespace, "vault auth path", loginAuthPath)

	vaultSecret, err := ReadVaultClientSecretFunc(vaultClient, string(jwt), role, loginAuthPath)
	if err != nil {
		return nil, fmt.Errorf("unable to obtain Vault client secret: %w", err)
	}

	if vaultSecret == nil || vaultSecret.Auth == nil || vaultSecret.Auth.ClientToken == "" {
		return nil, fmt.Errorf("no client token found in Vault secret")
	}

	vaultClient.SetToken(vaultSecret.Auth.ClientToken)
	return vaultSecret, nil
}

func renewToken(client *vault.Client, initialLoginErrorCh chan<- error) {
	logger := ctrl.LoggerFrom(nil)
	sentFirstLoginAttemptErr := false

	for {
		vaultLoginResp, err := login(client)
		if err != nil {
			logger.Error(err, "unable to authenticate to Vault server")
		}

		if !sentFirstLoginAttemptErr {
			initialLoginErrorCh <- err
			sentFirstLoginAttemptErr = true
			if err != nil {
				// Initial login attempt failed so fail fast and don't try to manage (non-existent) token lifecycle
				logger.Info("Lifecycle management of Vault token will not be carried out")
				return
			}
			logger.Info("Initiating lifecycle management of Vault token")
		}

		err = manageTokenLifecycle(client, vaultLoginResp)
		if err != nil {
			logger.Error(err, "unable to start managing the Vault token lifecycle")
		}

		// Reduce load on Vault server in a problem situation where repeated login attempts may be made
		time.Sleep(2 * time.Second)
	}
}

func manageTokenLifecycle(client *vault.Client, token *vault.Secret) error {
	logger := ctrl.LoggerFrom(nil)

	if token == nil || token.Auth == nil {
		logger.Info("No Vault secret available. Re-attempting login")
		return nil
	}

	renew := token.Auth.Renewable
	if !renew {
		logger.Info("Token is not configured to be renewable. Re-attempting login")
		return nil
	}

	watcher, err := client.NewLifetimeWatcher(&vault.LifetimeWatcherInput{
		Secret: token,
	})
	if err != nil {
		return fmt.Errorf("unable to initialize new lifetime watcher for renewing auth token: %w", err)
	}

	go watcher.Start()
	defer watcher.Stop()

	for {
		select {
		// `DoneCh` will return if renewal fails, or if the remaining lease duration is
		// under a built-in threshold and either renewing is not extending it or
		// renewing is disabled.  In any case, the caller needs to attempt to log in again.
		case err := <-watcher.DoneCh():
			if err != nil {
				logger.Error(err, "Failed to renew Vault token. Re-attempting login")
				return nil
			}
			logger.Info("Token can no longer be renewed. Re-attempting login.")
			return nil

		// Successfully completed renewal
		case renewal := <-watcher.RenewCh():
			logger.Info("Successfully renewed Vault token", "renewal info", renewal)
		}
	}
}

func ReadServiceAccountToken() ([]byte, error) {
	// Read the service-account token from the path where the token's Kubernetes Secret is mounted.
	// By default, Kubernetes will mount this to /var/run/secrets/kubernetes.io/serviceaccount/token
	// but an administrator may have configured it to be mounted elsewhere.
	path := "/var/run/secrets/kubernetes.io/serviceaccount/token"
	token, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("unable to read file %s: %w", path, err)
	}
	return token, nil
}

func ReadVaultClientSecret(vaultClient *vault.Client, jwtToken string, vaultRole string, authPath string) (*vault.Secret, error) {
	params := map[string]interface{}{
		"jwt":  jwtToken,
		"role": vaultRole, // the name of the role in Vault that was created with this app's Kubernetes service account bound to it
	}

	return LoginToVaultFunc(vaultClient, authPath, params)
}

func LoginToVault(vaultClient *vault.Client, authPath string, params map[string]interface{}) (*vault.Secret, error) {
	return vaultClient.Logical().Write(authPath+"/login", params)
}
