package rabbitmqclient_test

import (
	"errors"
	"github.com/rabbitmq/cluster-operator/internal/rabbitmqclient"
	"github.com/rabbitmq/cluster-operator/internal/rabbitmqclient/rabbitmqclientfakes"
	"os"

	vault "github.com/hashicorp/vault/api"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("VaultReader", func() {
	var (
		err                      error
		username, password       string
		secretStoreClient        rabbitmqclient.SecretStoreClient
		fakeSecretReader         *rabbitmqclientfakes.FakeSecretReader
		credsData                map[string]interface{}
		secretData               map[string]interface{}
		existingRabbitMQUsername = "abc123"
		existingRabbitMQPassword = "foo1234"
		vaultWarnings            []string
	)

	Describe("Read Credentials", func() {

		When("the credentials exist in the expected location", func() {
			BeforeEach(func() {
				credsData = make(map[string]interface{})
				secretData = make(map[string]interface{})
				credsData["username"] = existingRabbitMQUsername
				credsData["password"] = existingRabbitMQPassword
				secretData["data"] = credsData
				fakeSecretReader = &rabbitmqclientfakes.FakeSecretReader{}
				fakeSecretReader.ReadSecretReturns(&vault.Secret{Data: secretData}, nil)
				secretStoreClient = rabbitmqclient.VaultClient{Reader: fakeSecretReader}
			})

			JustBeforeEach(func() {
				username, password, err = secretStoreClient.ReadCredentials("some/path")
			})

			It("should return a credentials provider", func() {
				Expect(username).To(Equal(existingRabbitMQUsername))
				Expect(password).To(Equal(existingRabbitMQPassword))
			})

			It("should not error", func() {
				Expect(err).NotTo(HaveOccurred())
			})
		})

		When("unable to read secret from Vault", func() {
			BeforeEach(func() {
				err = errors.New("something bad happened")
				fakeSecretReader = &rabbitmqclientfakes.FakeSecretReader{}
				fakeSecretReader.ReadSecretReturns(nil, err)
				secretStoreClient = rabbitmqclient.VaultClient{Reader: fakeSecretReader}
			})

			JustBeforeEach(func() {
				username, password, err = secretStoreClient.ReadCredentials("some/path")
			})

			It("should return empty strings for username and password", func() {
				Expect(username).To(BeEmpty())
				Expect(password).To(BeEmpty())
			})

			It("should have returned an error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError("unable to read Vault secret: something bad happened"))
			})
		})

		When("Vault secret data does not contain expected map", func() {
			BeforeEach(func() {
				fakeSecretReader = &rabbitmqclientfakes.FakeSecretReader{}
				fakeSecretReader.ReadSecretReturns(&vault.Secret{}, nil)
				secretStoreClient = rabbitmqclient.VaultClient{Reader: fakeSecretReader}
			})

			JustBeforeEach(func() {
				username, password, err = secretStoreClient.ReadCredentials("some/path")
			})

			It("should return empty strings for username and password", func() {
				Expect(username).To(BeEmpty())
				Expect(password).To(BeEmpty())
			})

			It("should have returned an error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("returned Vault secret has a nil Data map"))
			})
		})

		When("Vault secret data contains an empty map", func() {
			BeforeEach(func() {
				secretData = make(map[string]interface{})
				fakeSecretReader = &rabbitmqclientfakes.FakeSecretReader{}
				fakeSecretReader.ReadSecretReturns(&vault.Secret{Data: secretData}, nil)
				secretStoreClient = rabbitmqclient.VaultClient{Reader: fakeSecretReader}
			})

			JustBeforeEach(func() {
				username, password, err = secretStoreClient.ReadCredentials("some/path")
			})

			It("should return empty strings for username and password", func() {
				Expect(username).To(BeEmpty())
				Expect(password).To(BeEmpty())
			})

			It("should have returned an error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError("returned Vault secret has an empty Data map"))
			})
		})

		When("Vault secret data map does not contain expected key/value entry", func() {
			BeforeEach(func() {
				secretData = make(map[string]interface{})
				secretData["somekey"] = "somevalue"
				fakeSecretReader = &rabbitmqclientfakes.FakeSecretReader{}
				fakeSecretReader.ReadSecretReturns(&vault.Secret{Data: secretData}, nil)
				secretStoreClient = rabbitmqclient.VaultClient{Reader: fakeSecretReader}
			})

			JustBeforeEach(func() {
				username, password, err = secretStoreClient.ReadCredentials("some/path")
			})

			It("should return empty strings for username and password", func() {
				Expect(username).To(BeEmpty())
				Expect(password).To(BeEmpty())
			})

			It("should have returned an error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError("returned Vault secret has a Data map that contains no value for key 'data'. Available keys are: [somekey]"))
			})
		})

		When("Vault secret data does not contain expected type", func() {
			BeforeEach(func() {
				secretData = make(map[string]interface{})
				secretData["data"] = "I am not a map"
				fakeSecretReader = &rabbitmqclientfakes.FakeSecretReader{}
				fakeSecretReader.ReadSecretReturns(&vault.Secret{Data: secretData}, nil)
				secretStoreClient = rabbitmqclient.VaultClient{Reader: fakeSecretReader}
			})

			JustBeforeEach(func() {
				username, password, err = secretStoreClient.ReadCredentials("some/path")
			})

			It("should return empty strings for username and password", func() {
				Expect(username).To(BeEmpty())
				Expect(password).To(BeEmpty())
			})

			It("should have returned an error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError("data type assertion failed for Vault secret of type: string and value \"I am not a map\" read from path some/path"))
			})
		})

		When("Vault secret data map does not contain username", func() {
			BeforeEach(func() {
				credsData = make(map[string]interface{})
				secretData = make(map[string]interface{})
				credsData["password"] = existingRabbitMQPassword
				secretData["data"] = credsData
				fakeSecretReader = &rabbitmqclientfakes.FakeSecretReader{}
				fakeSecretReader.ReadSecretReturns(&vault.Secret{Data: secretData}, nil)
				secretStoreClient = rabbitmqclient.VaultClient{Reader: fakeSecretReader}
			})

			JustBeforeEach(func() {
				username, password, err = secretStoreClient.ReadCredentials("some/path")
			})

			It("should return empty strings for username and password", func() {
				Expect(username).To(BeEmpty())
				Expect(password).To(BeEmpty())
			})

			It("should have returned an error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("unable to get username from Vault secret"))
			})
		})

		When("Vault secret data map does not contain password", func() {
			BeforeEach(func() {
				credsData = make(map[string]interface{})
				secretData = make(map[string]interface{})
				credsData["username"] = existingRabbitMQUsername
				secretData["data"] = credsData
				fakeSecretReader = &rabbitmqclientfakes.FakeSecretReader{}
				fakeSecretReader.ReadSecretReturns(&vault.Secret{Data: secretData}, nil)
				secretStoreClient = rabbitmqclient.VaultClient{Reader: fakeSecretReader}
			})

			JustBeforeEach(func() {
				username, password, err = secretStoreClient.ReadCredentials("some/path")
			})

			It("should return empty strings for username and password", func() {
				Expect(username).To(BeEmpty())
				Expect(password).To(BeEmpty())
			})

			It("should have returned an error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("unable to get password from Vault secret"))
			})
		})

		When("Vault secret data is nil", func() {
			BeforeEach(func() {
				fakeSecretReader = &rabbitmqclientfakes.FakeSecretReader{}
				fakeSecretReader.ReadSecretReturns(&vault.Secret{}, nil)
				secretStoreClient = rabbitmqclient.VaultClient{Reader: fakeSecretReader}
			})

			JustBeforeEach(func() {
				username, password, err = secretStoreClient.ReadCredentials("some/path")
			})

			It("should return empty strings for username and password", func() {
				Expect(username).To(BeEmpty())
				Expect(password).To(BeEmpty())
			})

			It("should have returned an error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("returned Vault secret has a nil Data map"))
			})
		})

		When("Vault secret is nil", func() {
			BeforeEach(func() {
				fakeSecretReader = &rabbitmqclientfakes.FakeSecretReader{}
				fakeSecretReader.ReadSecretReturns(nil, nil)
				secretStoreClient = rabbitmqclient.VaultClient{Reader: fakeSecretReader}
			})

			JustBeforeEach(func() {
				username, password, err = secretStoreClient.ReadCredentials("some/path")
			})

			It("should return empty strings for username and password", func() {
				Expect(username).To(BeEmpty())
				Expect(password).To(BeEmpty())
			})

			It("should have returned an error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("returned Vault secret is nil"))
			})
		})

		When("Vault secret contains warnings", func() {
			BeforeEach(func() {
				vaultWarnings = append(vaultWarnings, "something bad happened")
				credsData = make(map[string]interface{})
				secretData = make(map[string]interface{})
				credsData["password"] = existingRabbitMQPassword
				secretData["data"] = credsData
				fakeSecretReader = &rabbitmqclientfakes.FakeSecretReader{}
				fakeSecretReader.ReadSecretReturns(&vault.Secret{Data: secretData, Warnings: vaultWarnings}, nil)
				secretStoreClient = rabbitmqclient.VaultClient{Reader: fakeSecretReader}
			})

			JustBeforeEach(func() {
				username, password, err = secretStoreClient.ReadCredentials("some/path")
			})

			It("should return empty strings for username and password", func() {
				Expect(username).To(BeEmpty())
				Expect(password).To(BeEmpty())
			})

			It("should have returned an error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("warnings were returned from Vault"))
				Expect(err.Error()).To(ContainSubstring("something bad happened"))
			})
		})

	})

	Describe("Initialize secret store client", func() {
		var (
			getSecretStoreClientTester func() (rabbitmqclient.SecretStoreClient, error)
		)
		BeforeEach(func() {
			os.Setenv("VAULT_ADDR", "vault-address")
		})

		When("vault role is not set in the environment", func() {
			var vaultRoleUsedForLogin string

			BeforeEach(func() {
				rabbitmqclient.SecretClient = nil
				rabbitmqclient.SecretClientCreationError = nil
				rabbitmqclient.ReadServiceAccountTokenFunc = func() ([]byte, error) {
					return []byte("token"), nil
				}
				rabbitmqclient.LoginToVaultFunc = func(vaultClient *vault.Client, authPath string, params map[string]interface{}) (*vault.Secret, error) {
					return &vault.Secret{
						Auth: &vault.SecretAuth{
							ClientToken: "vault-secret-token",
						},
					}, nil
				}
				rabbitmqclient.ReadVaultClientSecretFunc = func(vaultClient *vault.Client, jwtToken string, vaultRole string, authPath string) (*vault.Secret, error) {
					vaultRoleUsedForLogin = vaultRole
					return &vault.Secret{
						Auth: &vault.SecretAuth{
							ClientToken: "vault-secret-token",
						},
					}, nil
				}
				getSecretStoreClientTester = func() (rabbitmqclient.SecretStoreClient, error) {
					rabbitmqclient.InitializeClient()()
					return rabbitmqclient.SecretClient, rabbitmqclient.SecretClientCreationError
				}
			})

			AfterEach(func() {
				rabbitmqclient.ReadServiceAccountTokenFunc = rabbitmqclient.ReadServiceAccountToken
				rabbitmqclient.LoginToVaultFunc = rabbitmqclient.LoginToVault
				rabbitmqclient.ReadVaultClientSecretFunc = rabbitmqclient.ReadVaultClientSecret
				vaultRoleUsedForLogin = ""
			})

			JustBeforeEach(func() {
				secretStoreClient, err = getSecretStoreClientTester()
			})

			It("should not error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should try to authenticate to Vault using the default Vault role value", func() {
				Expect(vaultRoleUsedForLogin).To(Equal("messaging-topology-operator"))
			})
		})

		When("vault role is set in the environment", func() {
			const operatorVaultRoleValue = "custom-role-value"
			var vaultRoleUsedForLogin string

			BeforeEach(func() {
				_ = os.Setenv("OPERATOR_VAULT_ROLE", operatorVaultRoleValue)
				rabbitmqclient.SecretClient = nil
				rabbitmqclient.SecretClientCreationError = nil
				rabbitmqclient.ReadServiceAccountTokenFunc = func() ([]byte, error) {
					return []byte("token"), nil
				}
				rabbitmqclient.LoginToVaultFunc = func(vaultClient *vault.Client, authPath string, params map[string]interface{}) (*vault.Secret, error) {
					return &vault.Secret{
						Auth: &vault.SecretAuth{
							ClientToken: "vault-secret-token",
						},
					}, nil
				}
				rabbitmqclient.ReadVaultClientSecretFunc = func(vaultClient *vault.Client, jwtToken string, vaultRole string, authPath string) (*vault.Secret, error) {
					vaultRoleUsedForLogin = vaultRole
					return &vault.Secret{
						Auth: &vault.SecretAuth{
							ClientToken: "vault-secret-token",
						},
					}, nil
				}
				getSecretStoreClientTester = func() (rabbitmqclient.SecretStoreClient, error) {
					rabbitmqclient.InitializeClient()()
					return rabbitmqclient.SecretClient, rabbitmqclient.SecretClientCreationError
				}
			})

			AfterEach(func() {
				rabbitmqclient.ReadServiceAccountTokenFunc = rabbitmqclient.ReadServiceAccountToken
				rabbitmqclient.LoginToVaultFunc = rabbitmqclient.LoginToVault
				rabbitmqclient.ReadVaultClientSecretFunc = rabbitmqclient.ReadVaultClientSecret
				vaultRoleUsedForLogin = ""
				_ = os.Unsetenv("OPERATOR_VAULT_ROLE")
			})

			JustBeforeEach(func() {
				secretStoreClient, err = getSecretStoreClientTester()
			})

			It("should not error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should try to authenticate to Vault using the Vault role value set with OPERATOR_VAULT_ROLE", func() {
				Expect(vaultRoleUsedForLogin).To(Equal(operatorVaultRoleValue))
			})
		})

		When("service account token is not in the expected place", func() {
			BeforeEach(func() {
				rabbitmqclient.SecretClient = nil
				rabbitmqclient.SecretClientCreationError = nil
				getSecretStoreClientTester = func() (rabbitmqclient.SecretStoreClient, error) {
					rabbitmqclient.InitializeClient()()
					return rabbitmqclient.SecretClient, rabbitmqclient.SecretClientCreationError
				}
			})

			JustBeforeEach(func() {
				secretStoreClient, err = getSecretStoreClientTester()
			})

			It("should return a nil secret store client", func() {
				Expect(secretStoreClient).To(BeNil())
			})

			It("should have returned an error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("unable to read file containing service account token"))
			})
		})

		When("unable to log into vault to obtain client secret", func() {
			BeforeEach(func() {
				rabbitmqclient.SecretClient = nil
				rabbitmqclient.SecretClientCreationError = nil
				rabbitmqclient.ReadServiceAccountTokenFunc = func() ([]byte, error) {
					return []byte("token"), nil
				}
				rabbitmqclient.LoginToVaultFunc = func(vaultClient *vault.Client, authPath string, params map[string]interface{}) (*vault.Secret, error) {
					return nil, errors.New("login failed (quickly!)")
				}
				getSecretStoreClientTester = func() (rabbitmqclient.SecretStoreClient, error) {
					rabbitmqclient.InitializeClient()()
					return rabbitmqclient.SecretClient, rabbitmqclient.SecretClientCreationError
				}
			})

			AfterEach(func() {
				rabbitmqclient.ReadServiceAccountTokenFunc = rabbitmqclient.ReadServiceAccountToken
				rabbitmqclient.LoginToVaultFunc = rabbitmqclient.LoginToVault
			})

			JustBeforeEach(func() {
				secretStoreClient, err = getSecretStoreClientTester()
			})

			It("should return a nil secret store client", func() {
				Expect(secretStoreClient).To(BeNil())
			})

			It("should have returned an error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("unable to obtain Vault client secret"))
			})
		})

		When("client secret obtained from vault", func() {
			BeforeEach(func() {
				rabbitmqclient.SecretClient = nil
				rabbitmqclient.SecretClientCreationError = nil
				rabbitmqclient.ReadServiceAccountTokenFunc = func() ([]byte, error) {
					return []byte("token"), nil
				}
				rabbitmqclient.LoginToVaultFunc = func(vaultClient *vault.Client, authPath string, params map[string]interface{}) (*vault.Secret, error) {
					return &vault.Secret{
						Auth: &vault.SecretAuth{
							ClientToken: "vault-secret-token",
						},
					}, nil
				}
				rabbitmqclient.ReadVaultClientSecretFunc = func(vaultClient *vault.Client, jwtToken string, vaultRole string, authPath string) (*vault.Secret, error) {
					return &vault.Secret{
						Auth: &vault.SecretAuth{
							ClientToken: "vault-secret-token",
						},
					}, nil
				}
				getSecretStoreClientTester = func() (rabbitmqclient.SecretStoreClient, error) {
					rabbitmqclient.InitializeClient()()
					return rabbitmqclient.SecretClient, rabbitmqclient.SecretClientCreationError
				}
			})

			AfterEach(func() {
				rabbitmqclient.ReadServiceAccountTokenFunc = rabbitmqclient.ReadServiceAccountToken
				rabbitmqclient.LoginToVaultFunc = rabbitmqclient.LoginToVault
				rabbitmqclient.ReadVaultClientSecretFunc = rabbitmqclient.ReadVaultClientSecret
			})

			JustBeforeEach(func() {
				secretStoreClient, err = getSecretStoreClientTester()
			})

			It("should not error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return a secret store client", func() {
				Expect(secretStoreClient).ToNot(BeNil())
			})
		})

		When("VAULT_ADDR is not set", func() {
			BeforeEach(func() {
				getSecretStoreClientTester = func() (rabbitmqclient.SecretStoreClient, error) {
					rabbitmqclient.InitializeClient()()
					return rabbitmqclient.SecretClient, rabbitmqclient.SecretClientCreationError
				}
			})
			It("returns an error", func() {
				os.Unsetenv("VAULT_ADDR")
				secretStoreClient, err = getSecretStoreClientTester()
				Expect(err).To(MatchError("VAULT_ADDR environment variable not set; cannot initialize vault client"))
			})
		})
	})
})
