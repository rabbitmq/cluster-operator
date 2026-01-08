package controllers

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"sync"

	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/v2/api/v1beta1"
	"github.com/rabbitmq/cluster-operator/v2/internal/rabbitmqclient"
	"github.com/rabbitmq/cluster-operator/v2/internal/resource"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// nodeQuorumCheck holds the result of a quorum check for a single node
type nodeQuorumCheck struct {
	podName string
	status  string // "ok", "quorum-critical", "unavailable"
	err     error
}

// reconcileStatus sets status.defaultUser (secret and service reference) and status.binding.
// when vault is used as secret backend for default user, no user secret object is created
// therefore only status.defaultUser.serviceReference is set.
// status.binding exposes the default user secret which contains the binding
// information for this RabbitmqCluster.
// Default user secret implements the service binding Provisioned Service
// See: https://k8s-service-bindings.github.io/spec/#provisioned-service
func (r *RabbitmqClusterReconciler) reconcileStatus(ctx context.Context, rmq *rabbitmqv1beta1.RabbitmqCluster) error {
	var binding *corev1.LocalObjectReference

	defaultUserStatus := &rabbitmqv1beta1.RabbitmqClusterDefaultUser{
		ServiceReference: &rabbitmqv1beta1.RabbitmqClusterServiceReference{
			Name:      rmq.ChildResourceName(""),
			Namespace: rmq.Namespace,
		},
	}

	if !rmq.VaultDefaultUserSecretEnabled() {
		defaultUserStatus.SecretReference = &rabbitmqv1beta1.RabbitmqClusterSecretReference{
			Name:      rmq.ChildResourceName(resource.DefaultUserSecretName),
			Namespace: rmq.Namespace,
			Keys: map[string]string{
				"username": "username",
				"password": "password",
			},
		}
		if !rmq.ExternalSecretEnabled() {
			binding = &corev1.LocalObjectReference{
				Name: rmq.ChildResourceName(resource.DefaultUserSecretName),
			}
		} else {
			binding = &corev1.LocalObjectReference{
				Name: rmq.Spec.SecretBackend.ExternalSecret.Name,
			}

		}
	}

	if !reflect.DeepEqual(rmq.Status.DefaultUser, defaultUserStatus) || !reflect.DeepEqual(rmq.Status.Binding, binding) {
		rmq.Status.DefaultUser = defaultUserStatus
		rmq.Status.Binding = binding
		if err := r.Status().Update(ctx, rmq); err != nil {
			return err
		}
	}

	return nil
}

// getPodEndpoints retrieves all pod endpoints from the EndpointSlice
func (r *RabbitmqClusterReconciler) getPodEndpoints(ctx context.Context, rmq *rabbitmqv1beta1.RabbitmqCluster) ([]discoveryv1.Endpoint, error) {
	endpointSliceList := &discoveryv1.EndpointSliceList{}

	// Use the same label selector as the main service
	selector, err := labels.Parse(fmt.Sprintf("%s=%s", discoveryv1.LabelServiceName, rmq.Name))
	if err != nil {
		return nil, fmt.Errorf("failed to parse label selector: %w", err)
	}

	listOptions := client.ListOptions{
		LabelSelector: selector,
		Namespace:     rmq.Namespace,
	}

	if err := r.List(ctx, endpointSliceList, &listOptions); err != nil {
		return nil, fmt.Errorf("failed to list endpoint slices: %w", err)
	}

	if len(endpointSliceList.Items) == 0 {
		return nil, fmt.Errorf("no endpoint slices found for cluster %s", rmq.Name)
	}

	// Return endpoints from the first EndpointSlice
	// In most cases there's only one EndpointSlice per service
	return endpointSliceList.Items[0].Endpoints, nil
}

// checkNodeQuorumStatus checks quorum status for a specific node/pod
func (r *RabbitmqClusterReconciler) checkNodeQuorumStatus(ctx context.Context, rmq *rabbitmqv1beta1.RabbitmqCluster, podIP string, podName string) nodeQuorumCheck {
	logger := ctrl.LoggerFrom(ctx)

	// Get client for this specific pod
	rabbitClient, err := rabbitmqclient.GetRabbitmqClientForPod(ctx, r.APIReader, rmq, podIP)
	if err != nil {
		logger.V(1).Info("Failed to get client for pod", "pod", podName, "error", err)
		return nodeQuorumCheck{
			podName: podName,
			status:  "unavailable",
			err:     err,
		}
	}

	// Check quorum status
	result, err := rabbitClient.HealthCheckNodeIsQuorumCritical()
	if err != nil {
		logger.V(1).Info("Quorum health check failed for pod", "pod", podName, "error", err)
		return nodeQuorumCheck{
			podName: podName,
			status:  "unavailable",
			err:     err,
		}
	}

	if result.Ok() {
		return nodeQuorumCheck{
			podName: podName,
			status:  "ok",
			err:     nil,
		}
	}

	return nodeQuorumCheck{
		podName: podName,
		status:  "quorum-critical",
		err:     nil,
	}
}

// checkQuorumStatus checks if any RabbitMQ node is quorum critical by checking all nodes.
// Returns a formatted status string with details about critical nodes and unavailable nodes.
func (r *RabbitmqClusterReconciler) checkQuorumStatus(ctx context.Context, rmq *rabbitmqv1beta1.RabbitmqCluster) string {
	logger := ctrl.LoggerFrom(ctx)

	// Get all pod endpoints
	endpoints, err := r.getPodEndpoints(ctx, rmq)
	if err != nil {
		logger.Info("Failed to get pod endpoints for quorum check", "error", err)
		return "unavailable"
	}

	if len(endpoints) == 0 {
		logger.Info("No endpoints found for quorum check")
		return "unavailable"
	}

	// Check all nodes concurrently
	var wg sync.WaitGroup
	resultsChan := make(chan nodeQuorumCheck, len(endpoints))

	for _, endpoint := range endpoints {
		// Get pod name from hostname or targetRef
		podName := "unknown"
		if endpoint.Hostname != nil && *endpoint.Hostname != "" {
			podName = *endpoint.Hostname
		} else if endpoint.TargetRef != nil && endpoint.TargetRef.Name != "" {
			podName = endpoint.TargetRef.Name
		}

		// Get pod IP
		if len(endpoint.Addresses) == 0 {
			logger.V(1).Info("Endpoint has no addresses", "podName", podName)
			resultsChan <- nodeQuorumCheck{
				podName: podName,
				status:  "unavailable",
				err:     fmt.Errorf("no addresses for endpoint"),
			}
			continue
		}

		podIP := endpoint.Addresses[0]

		wg.Add(1)
		go func(ip, name string) {
			defer wg.Done()
			result := r.checkNodeQuorumStatus(ctx, rmq, ip, name)
			resultsChan <- result
		}(podIP, podName)
	}

	// Wait for all checks to complete
	wg.Wait()
	close(resultsChan)

	// Aggregate results
	var criticalPods []string
	var unavailableCount int
	var okCount int

	for result := range resultsChan {
		switch result.status {
		case "quorum-critical":
			criticalPods = append(criticalPods, result.podName)
		case "unavailable":
			unavailableCount++
		case "ok":
			okCount++
		}
	}

	// Format the status string
	if len(criticalPods) > 0 {
		status := fmt.Sprintf("quorum-critical: %s", strings.Join(criticalPods, ", "))
		if unavailableCount > 0 {
			status = fmt.Sprintf("%s (%d unavailable)", status, unavailableCount)
		}
		return status
	}

	// No critical nodes
	if okCount > 0 {
		if unavailableCount > 0 {
			return fmt.Sprintf("ok (%d unavailable)", unavailableCount)
		}
		return "ok"
	}

	// All nodes unavailable
	return "unavailable"
}

// updateQuorumStatus updates the QuorumStatus field in the cluster status.
func (r *RabbitmqClusterReconciler) updateQuorumStatus(ctx context.Context, rmq *rabbitmqv1beta1.RabbitmqCluster) error {
	logger := ctrl.LoggerFrom(ctx)

	// Check current quorum status
	newStatus := r.checkQuorumStatus(ctx, rmq)

	// Only update if the status has changed
	if rmq.Status.QuorumStatus == newStatus {
		return nil
	}

	// Update the status
	rmq.Status.QuorumStatus = newStatus
	if err := r.Status().Update(ctx, rmq); err != nil {
		// If it's a conflict error, just log it - we'll retry on the next reconciliation
		if k8serrors.IsConflict(err) {
			logger.Info("Failed to update quorum status due to conflict; will retry on next reconciliation",
				"namespace", rmq.Namespace,
				"name", rmq.Name)
			return nil
		}
		return fmt.Errorf("failed to update quorum status: %w", err)
	}

	logger.Info("Updated quorum status", "status", newStatus)
	return nil
}
