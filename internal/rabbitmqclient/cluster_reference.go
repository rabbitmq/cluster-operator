package rabbitmqclient

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/api/v1beta1"
	"gopkg.in/ini.v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ClusterCredentials struct {
	data map[string][]byte
}

func (c ClusterCredentials) Data(key string) ([]byte, bool) {
	result, ok := c.data[key]
	return result, ok
}

var SecretStoreClientProvider = GetSecretStoreClient

var (
	NoSuchRabbitmqClusterError = errors.New("RabbitmqCluster object does not exist")
	ResourceNotAllowedError    = errors.New("resource is not allowed to reference defined cluster reference. Check the namespace of the resource is allowed as part of the cluster's `rabbitmq.com/topology-allowed-namespaces` annotation")
	NoServiceReferenceSetError = errors.New("RabbitmqCluster has no ServiceReference set in status.defaultUser")
)

func ParseReference(ctx context.Context, c client.Client, rmq rabbitmqv1beta1.RabbitmqClusterReference, requestNamespace string, clusterDomain string) (map[string]string, bool, error) {
	if rmq.ConnectionSecret != nil {
		secret := &corev1.Secret{}
		if err := c.Get(ctx, types.NamespacedName{Namespace: requestNamespace, Name: rmq.ConnectionSecret.Name}, secret); err != nil {
			return nil, false, err
		}
		return readCredentialsFromKubernetesSecret(secret)
	}

	var namespace string
	if rmq.Namespace == "" {
		namespace = requestNamespace
	} else {
		namespace = rmq.Namespace
	}

	cluster := &rabbitmqv1beta1.RabbitmqCluster{}
	if err := c.Get(ctx, types.NamespacedName{Name: rmq.Name, Namespace: namespace}, cluster); err != nil {
		return nil, false, fmt.Errorf("failed to get cluster from reference: %s Error: %w", err, NoSuchRabbitmqClusterError)
	}

	if !AllowedNamespace(rmq, requestNamespace, cluster) {
		return nil, false, ResourceNotAllowedError
	}

	if cluster.Status.DefaultUser == nil || cluster.Status.DefaultUser.ServiceReference == nil {
		return nil, false, NoServiceReferenceSetError
	}

	var user, pass string
	if cluster.Spec.SecretBackend.Vault != nil && cluster.Spec.SecretBackend.Vault.DefaultUserPath != "" {
		// ask the configured secure store for the credentials available at the path retrieved from the cluster resource
		secretStoreClient, err := SecretStoreClientProvider()
		if err != nil {
			return nil, false, fmt.Errorf("unable to create a client connection to secret store: %w", err)
		}

		user, pass, err = secretStoreClient.ReadCredentials(cluster.Spec.SecretBackend.Vault.DefaultUserPath)
		if err != nil {
			return nil, false, fmt.Errorf("unable to retrieve credentials from secret store: %w", err)
		}
	} else {
		// use credentials in namespace Kubernetes Secret
		if cluster.Status.Binding == nil {
			return nil, false, errors.New("no status.binding set")
		}

		secret := &corev1.Secret{}
		if err := c.Get(ctx, types.NamespacedName{Namespace: namespace, Name: cluster.Status.Binding.Name}, secret); err != nil {
			return nil, false, err
		}
		var err error
		user, pass, err = readUsernamePassword(secret)
		if err != nil {
			return nil, false, fmt.Errorf("unable to retrieve credentials from Kubernetes secret %s: %w", secret.Name, err)
		}
	}

	svc := &corev1.Service{}
	if err := c.Get(ctx, types.NamespacedName{Namespace: namespace, Name: cluster.Status.DefaultUser.ServiceReference.Name}, svc); err != nil {
		return nil, false, err
	}

	additionalConfig, err := readClusterAdditionalConfig(cluster)
	if err != nil {
		return nil, false, fmt.Errorf("unable to parse additionconfig setting from the rabbitmqcluster resource: %w", err)
	}

	endpoint, err := managementURI(svc, cluster.TLSEnabled(), clusterDomain, additionalConfig["management.path_prefix"])
	if err != nil {
		return nil, false, fmt.Errorf("failed to get endpoint from specified rabbitmqcluster: %w", err)
	}

	return map[string]string{
		"username": user,
		"password": pass,
		"uri":      endpoint,
	}, cluster.TLSEnabled(), nil
}

func AllowedNamespace(rmq rabbitmqv1beta1.RabbitmqClusterReference, requestNamespace string, cluster *rabbitmqv1beta1.RabbitmqCluster) bool {
	if rmq.Namespace != "" && rmq.Namespace != requestNamespace {
		var isAllowed bool
		if allowedNamespaces, ok := cluster.Annotations["rabbitmq.com/topology-allowed-namespaces"]; ok {
			for _, allowedNamespace := range strings.Split(allowedNamespaces, ",") {
				if requestNamespace == allowedNamespace || allowedNamespace == "*" {
					isAllowed = true
					break
				}
			}
		}
		if !isAllowed {
			return false
		}
	}
	return true
}

func readCredentialsFromKubernetesSecret(secret *corev1.Secret) (map[string]string, bool, error) {
	if secret == nil {
		return nil, false, fmt.Errorf("unable to retrieve information from Kubernetes secret %s: %w", secret.Name, errors.New("nil secret"))
	}

	uBytes, found := secret.Data["uri"]
	if !found {
		return nil, false, keyMissingErr("uri")
	}

	uri := string(uBytes)
	if !strings.HasPrefix(uri, "http") {
		uri = "http://" + uri // set scheme to http if not provided
	}
	var tlsEnabled bool
	if parsed, err := url.Parse(uri); err != nil {
		return nil, false, err
	} else if parsed.Scheme == "https" {
		tlsEnabled = true
	}

	return map[string]string{
		"username": string(secret.Data["username"]),
		"password": string(secret.Data["password"]),
		"uri":      uri,
	}, tlsEnabled, nil
}

func readClusterAdditionalConfig(cluster *rabbitmqv1beta1.RabbitmqCluster) (additionalConfig map[string]string, err error) {
	cfg, err := ini.Load([]byte(cluster.Spec.Rabbitmq.AdditionalConfig))
	if err != nil {
		return
	}

	// Additional config has only a default section
	additionalConfig = cfg.Section("").KeysHash()

	return
}

func readUsernamePassword(secret *corev1.Secret) (string, string, error) {
	if secret == nil {
		return "", "", errors.New("unable to extract data from nil secret")
	}

	return string(secret.Data["username"]), string(secret.Data["password"]), nil
}

func managementURI(svc *corev1.Service, tlsEnabled bool, clusterDomain string, pathPrefix string) (string, error) {
	var managementUiPort int
	for _, port := range svc.Spec.Ports {
		if port.Name == "management-tls" {
			managementUiPort = int(port.Port)
			break
		}
		if port.Name == "management" {
			managementUiPort = int(port.Port)
			// Do not break here because we may still find 'management-tls' port
		}
	}

	if managementUiPort == 0 {
		return "", fmt.Errorf("failed to find 'management' or 'management-tls' from service %s", svc.Name)
	}

	scheme := "http"
	if tlsEnabled {
		scheme = "https"
	}
	url := url.URL{
		Scheme: scheme,
		Host:   fmt.Sprintf("%s.%s.svc%s:%d", svc.Name, svc.Namespace, clusterDomain, managementUiPort),
		Path:   pathPrefix,
	}
	return url.String(), nil
}
