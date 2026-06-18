// RabbitMQ Cluster Operator
//
// Copyright 2020 VMware, Inc. All Rights Reserved.
//
// This product is licensed to you under the Mozilla Public license, Version 2.0 (the "License").  You may not use this product except in compliance with the Mozilla Public License.
//
// This product may include a number of subcomponents with separate copyright notices and license terms. Your use of these subcomponents is subject to the terms and conditions of the subcomponent's license, as noted in the LICENSE file.
//

package system_test

import (
	"context"
	"os"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/v2/api/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	defaultscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestSystemTests(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "SystemTests Suite")
}

var (
	rmqClusterClient     client.Client
	clientSet            *kubernetes.Clientset
	namespace            string
	operatorNamespace    string
	storageClassName     = "persistent-test"
	namespaceCreatedByUs bool
)

var _ = SynchronizedBeforeSuite(
	// This function runs ONLY on parallel node 1
	func() []byte {
		ctx := context.Background()

		namespace = getEnvOrDefault("NAMESPACE", "cluster-operator-system-tests")
		operatorNamespace = MustHaveEnv("K8S_OPERATOR_NAMESPACE")

		// Create test namespace if it doesn't exist and differs from operator namespace
		if namespace != operatorNamespace {
			restConfig := controllerruntime.GetConfigOrDie()
			cs, err := kubernetes.NewForConfig(restConfig)
			Expect(err).NotTo(HaveOccurred())

			_, err = cs.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
			if err != nil {
				if errors.IsNotFound(err) {
					By("Creating test namespace: " + namespace)
					ns := &corev1.Namespace{
						ObjectMeta: metav1.ObjectMeta{
							Name: namespace,
						},
					}
					_, err = cs.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
					Expect(err).NotTo(HaveOccurred())
					return []byte("created") // Signal that we created the namespace
				}
				Expect(err).NotTo(HaveOccurred())
			}
		}
		return []byte("") // Namespace already existed or same as operator namespace
	},
	// This function runs on ALL parallel nodes (including node 1)
	func(namespaceCreationStatus []byte) {
		namespaceCreatedByUs = string(namespaceCreationStatus) == "created"

		scheme := runtime.NewScheme()
		Expect(rabbitmqv1beta1.AddToScheme(scheme)).To(Succeed())
		Expect(defaultscheme.AddToScheme(scheme)).To(Succeed())

		restConfig := controllerruntime.GetConfigOrDie()

		var err error
		rmqClusterClient, err = client.New(restConfig, client.Options{Scheme: scheme})
		Expect(err).NotTo(HaveOccurred())
		clientSet, err = createClientSet()
		Expect(err).NotTo(HaveOccurred())

		namespace = getEnvOrDefault("NAMESPACE", "cluster-operator-system-tests")
		operatorNamespace = MustHaveEnv("K8S_OPERATOR_NAMESPACE")

		ctx := context.Background()
		Eventually(func() int32 {
			operatorDeployment, err := clientSet.AppsV1().Deployments(operatorNamespace).Get(ctx, "rabbitmq-cluster-operator", metav1.GetOptions{})
			ExpectWithOffset(1, err).NotTo(HaveOccurred())
			return operatorDeployment.Status.ReadyReplicas
		}, 10, 1).Should(BeNumerically("==", 1), "Expected to have Operator Pod Ready")

		// Wait for the mutating webhook CA bundle to be injected by cert-manager
		Eventually(func() string {
			mwc, err := clientSet.AdmissionregistrationV1().MutatingWebhookConfigurations().Get(ctx, "mutating-webhook-configuration", metav1.GetOptions{})
			if err != nil || len(mwc.Webhooks) == 0 {
				return ""
			}
			return string(mwc.Webhooks[0].ClientConfig.CABundle)
		}, 60, 1).ShouldNot(BeEmpty(), "Expected MutatingWebhookConfiguration CA bundle to be injected")
	},
)

var _ = SynchronizedAfterSuite(
	// This function runs on ALL parallel nodes first
	func() {
		ctx := context.Background()
		_ = clientSet.StorageV1().StorageClasses().Delete(ctx, storageClassName, metav1.DeleteOptions{})
	},
	// This function runs ONLY on parallel node 1, after all other nodes complete
	func() {
		if namespaceCreatedByUs && namespace != operatorNamespace {
			ctx := context.Background()
			By("Deleting test namespace: " + namespace)
			restConfig := controllerruntime.GetConfigOrDie()
			cs, err := kubernetes.NewForConfig(restConfig)
			if err != nil {
				GinkgoWriter.Printf("Warning: Failed to create clientset for cleanup: %v\n", err)
				return
			}
			err = cs.CoreV1().Namespaces().Delete(ctx, namespace, metav1.DeleteOptions{})
			if err != nil && !errors.IsNotFound(err) {
				GinkgoWriter.Printf("Warning: Failed to delete test namespace %s: %v\n", namespace, err)
			}
		}
	},
)

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
