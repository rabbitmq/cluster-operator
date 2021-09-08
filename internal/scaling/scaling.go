package scaling

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	k8sresource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type PersistenceScaler struct {
	Client kubernetes.Interface
}

func NewPersistenceScaler(client kubernetes.Interface) PersistenceScaler {
	return PersistenceScaler{
		Client: client,
	}
}

func (p PersistenceScaler) Scale(ctx context.Context, existingCluster rabbitmqv1beta1.RabbitmqCluster, desiredCapacity k8sresource.Quantity) error {
	logger := ctrl.LoggerFrom(ctx)

	existingPVCs, err := p.getClusterPVCs(ctx, existingCluster)
	if err != nil {
		logger.Error(err, "failed to retrieve the existing cluster PVCs")
		return err
	}
	pvcsToBeScaled, err := p.pvcsNeedingScaling(ctx, existingPVCs, desiredCapacity)
	if err != nil {
		logger.Error(err, "did not complete scaling actions")
		return err
	}

	if len(pvcsToBeScaled) == 0 {
		logger.Info("No PVC for this RabbitmqCluster requires scaling", "RabbitmqCluster", existingCluster.Name)
		return nil
	}
	logger.Info("Scaling up PVCs for a RabbitmqCluster", "RabbitmqCluster", existingCluster.Name, "pvcsToBeScaled", pvcsToBeScaled)

	err = p.deleteSts(ctx, existingCluster)
	if err != nil {
		logErr := fmt.Errorf("Failed to delete Statefulset from Kubernetes API: %w", err)
		logger.Error(logErr, "Could not delete existing sts")
		return logErr
	}

	return p.scaleUpPVCs(ctx, existingCluster, pvcsToBeScaled, desiredCapacity)
}

func (p PersistenceScaler) getClusterPVCs(ctx context.Context, existingCluster rabbitmqv1beta1.RabbitmqCluster) ([]*corev1.PersistentVolumeClaim, error) {
	logger := ctrl.LoggerFrom(ctx)

	var pvcs []*corev1.PersistentVolumeClaim

	for i := 0; i < int(*existingCluster.Spec.Replicas); i++ {
		pvc, err := p.Client.CoreV1().PersistentVolumeClaims(existingCluster.Namespace).Get(ctx, existingCluster.PVCName(i), metav1.GetOptions{})
		if client.IgnoreNotFound(err) != nil {
			logErr := fmt.Errorf("Failed to get PVC from Kubernetes API: %w", err)
			logger.Error(logErr, "Could not read existing PVC")
			return nil, logErr
		}
		// If the PVC exists, we may need to scale it.
		if err == nil {
			pvcs = append(pvcs, pvc)
		}
	}
	return pvcs, nil
}

func (p PersistenceScaler) pvcsNeedingScaling(ctx context.Context, existingPVCs []*corev1.PersistentVolumeClaim, desiredCapacity k8sresource.Quantity) ([]*corev1.PersistentVolumeClaim, error) {
	logger := ctrl.LoggerFrom(ctx)

	var pvcs []*corev1.PersistentVolumeClaim

	for _, pvc := range existingPVCs {
		existingCapacity := pvc.Spec.Resources.Requests[corev1.ResourceStorage]
		cmp := existingCapacity.Cmp(desiredCapacity)

		// desired storage capacity is larger than the current capacity; PVC needs expansion
		if cmp == -1 {
			pvcs = append(pvcs, pvc)
		}

		// desired storage capacity is smaller than the current capacity; we can't proceed lest we lose data
		if cmp == 1 {
			msg := "shrinking persistent volumes is not supported"
			logger.Error(errors.New("unsupported operation"), msg)
			return pvcs, errors.New(msg)
		}

		// don't allow going from 0 (no PVC) to anything else
		if (existingCapacity.Cmp(k8sresource.MustParse("0Gi")) == 0) && (desiredCapacity.Cmp(k8sresource.MustParse("0Gi")) != 0) {
			msg := "changing from ephemeral to persistent storage is not supported"
			logger.Error(errors.New("unsupported operation"), msg)
			return pvcs, errors.New(msg)
		}

	}
	return pvcs, nil
}

// deleteSts deletes a sts without deleting pods and PVCs
// using DeletePropagationPolicy set to 'Orphan'
func (p PersistenceScaler) deleteSts(ctx context.Context, existingCluster rabbitmqv1beta1.RabbitmqCluster) error {
	logger := ctrl.LoggerFrom(ctx)
	logger.Info("deleting statefulSet (pods won't be deleted)", "statefulSet", existingCluster.ChildResourceName("server"))

	sts, err := p.Client.AppsV1().StatefulSets(existingCluster.Namespace).Get(ctx, existingCluster.ChildResourceName("server"), metav1.GetOptions{})
	if client.IgnoreNotFound(err) != nil {
		logErr := fmt.Errorf("Failed to get statefulset from Kubernetes API: %w", err)
		logger.Error(logErr, "Could not read existing statefulset")
		return logErr
	}

	// The StatefulSet may have already been deleted. If so, there is no need to delete it again.
	if k8serrors.IsNotFound(err) {
		logger.Info("statefulset has already been deleted", "StatefulSet", existingCluster.Name, "RabbitmqCluster", existingCluster.Name)
		return nil
	}

	deletePropagationPolicy := metav1.DeletePropagationOrphan
	if err = p.Client.AppsV1().StatefulSets(sts.Namespace).Delete(ctx, sts.Name, metav1.DeleteOptions{PropagationPolicy: &deletePropagationPolicy}); err != nil {
		msg := "failed to delete statefulSet"
		logger.Error(err, msg, "statefulSet", sts.Name)
		return fmt.Errorf("%s %s: %w", msg, sts.Name, err)
	}

	if err := retryWithInterval(logger, "delete statefulSet", 10, 3*time.Second, func() bool {
		_, getErr := p.Client.AppsV1().StatefulSets(existingCluster.Namespace).Get(ctx, existingCluster.ChildResourceName("server"), metav1.GetOptions{})
		return k8serrors.IsNotFound(getErr)
	}); err != nil {
		msg := "statefulSet not deleting after 30 seconds"
		logger.Error(err, msg, "statefulSet", sts.Name)
		return fmt.Errorf("%s %s: %w", msg, sts.Name, err)
	}
	logger.Info("statefulSet deleted", "statefulSet", sts.Name)
	return nil
}

func (p PersistenceScaler) scaleUpPVCs(ctx context.Context, existingCluster rabbitmqv1beta1.RabbitmqCluster, pvcs []*corev1.PersistentVolumeClaim, desiredCapacity k8sresource.Quantity) error {
	logger := ctrl.LoggerFrom(ctx)

	for _, pvc := range pvcs {
		// To minimise any timing windows, retrieve the latest version of this PVC before updating
		pvc, err := p.Client.CoreV1().PersistentVolumeClaims(pvc.Namespace).Get(ctx, pvc.Name, metav1.GetOptions{})
		if err != nil {
			logErr := fmt.Errorf("Failed to get PVC from Kubernetes API: %w", err)
			logger.Error(logErr, "Could not read existing PVC")
			return logErr
		}

		pvc.Spec.Resources.Requests[corev1.ResourceStorage] = desiredCapacity
		_, err = p.Client.CoreV1().PersistentVolumeClaims(existingCluster.Namespace).Update(ctx, pvc, metav1.UpdateOptions{})
		if err != nil {
			msg := "failed to update PersistentVolumeClaim"
			logger.Error(err, msg, "PersistentVolumeClaim", pvc.Name)
			return fmt.Errorf("%s %s: %w", msg, pvc.Name, err)
		}
		logger.Info("Successfully scaled up PVC", "PersistentVolumeClaim", pvc.Name, "newCapacity", desiredCapacity)
	}
	return nil
}

func retryWithInterval(logger logr.Logger, msg string, retry int, interval time.Duration, f func() bool) (err error) {
	for i := 0; i < retry; i++ {
		if ok := f(); ok {
			return
		}
		time.Sleep(interval)
		logger.Info("retrying again", "action", msg, "interval", interval, "attempt", i+1)
	}
	return fmt.Errorf("failed to %s after %d retries", msg, retry)
}
