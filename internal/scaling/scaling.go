package scaling

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/api/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	k8sresource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/pointer"
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

func (p PersistenceScaler) Scale(ctx context.Context, rmq rabbitmqv1beta1.RabbitmqCluster, desiredCapacity k8sresource.Quantity) error {
	logger := ctrl.LoggerFrom(ctx)

	existingCapacity, err := p.existingCapacity(ctx, rmq)
	if client.IgnoreNotFound(err) != nil {
		logErr := fmt.Errorf("failed to determine existing STS capactiy: %w", err)
		logger.Error(logErr, "Could not read sts")
		return logErr
	}

	// don't allow going from 0 (no PVC) to anything else
	if err == nil && (existingCapacity.Cmp(k8sresource.MustParse("0Gi")) == 0) && (desiredCapacity.Cmp(k8sresource.MustParse("0Gi")) != 0) {
		msg := "changing from ephemeral to persistent storage is not supported"
		logger.Error(errors.New("unsupported operation"), msg)
		return errors.New(msg)
	}

	// desired storage capacity is smaller than the current capacity; we can't proceed lest we lose data
	if err == nil && existingCapacity.Cmp(desiredCapacity) == 1 {
		msg := "shrinking persistent volumes is not supported"
		logger.Error(errors.New("unsupported operation"), msg)
		return errors.New(msg)
	}

	existingPVCs, err := p.getClusterPVCs(ctx, rmq)
	if err != nil {
		logger.Error(err, "failed to retrieve the existing cluster PVCs")
		return err
	}
	pvcsToBeScaled := p.pvcsNeedingScaling(existingPVCs, desiredCapacity)
	if len(pvcsToBeScaled) == 0 {
		return nil
	}
	logger.Info("Scaling up PVCs", "RabbitmqCluster", rmq.Name, "pvcsToBeScaled", pvcsToBeScaled)

	if err := p.deleteSts(ctx, rmq); err != nil {
		logErr := fmt.Errorf("failed to delete Statefulset from Kubernetes API: %w", err)
		logger.Error(logErr, "Could not delete existing sts")
		return logErr
	}

	return p.scaleUpPVCs(ctx, rmq, pvcsToBeScaled, desiredCapacity)
}

func (p PersistenceScaler) getClusterPVCs(ctx context.Context, rmq rabbitmqv1beta1.RabbitmqCluster) ([]*corev1.PersistentVolumeClaim, error) {
	logger := ctrl.LoggerFrom(ctx)

	var pvcs []*corev1.PersistentVolumeClaim

	var i int32
	for i = 0; i < pointer.Int32Deref(rmq.Spec.Replicas, 1); i++ {
		pvc, err := p.Client.CoreV1().PersistentVolumeClaims(rmq.Namespace).Get(ctx, rmq.PVCName(int(i)), metav1.GetOptions{})
		if client.IgnoreNotFound(err) != nil {
			logErr := fmt.Errorf("failed to get PVC from Kubernetes API: %w", err)
			logger.Error(logErr, "Could not read existing PVC")
			return nil, logErr
		}
		// If the PVC exists, we may need to scale it.
		if err == nil {
			pvcs = append(pvcs, pvc)
		}
	}
	if len(pvcs) > 0 {
		logger.V(1).Info("Found existing PVCs", "pvcList", pvcs)
	}
	return pvcs, nil
}

func (p PersistenceScaler) pvcsNeedingScaling(existingPVCs []*corev1.PersistentVolumeClaim, desiredCapacity k8sresource.Quantity) []*corev1.PersistentVolumeClaim {
	var pvcs []*corev1.PersistentVolumeClaim

	for _, pvc := range existingPVCs {
		existingCapacity := pvc.Spec.Resources.Requests[corev1.ResourceStorage]

		// desired storage capacity is larger than the current capacity; PVC needs expansion
		if existingCapacity.Cmp(desiredCapacity) == -1 {
			pvcs = append(pvcs, pvc)
		}
	}
	return pvcs
}

func (p PersistenceScaler) getSts(ctx context.Context, rmq rabbitmqv1beta1.RabbitmqCluster) (*appsv1.StatefulSet, error) {
	return p.Client.AppsV1().StatefulSets(rmq.Namespace).Get(ctx, rmq.ChildResourceName("server"), metav1.GetOptions{})
}

func (p PersistenceScaler) existingCapacity(ctx context.Context, rmq rabbitmqv1beta1.RabbitmqCluster) (k8sresource.Quantity, error) {
	sts, err := p.getSts(ctx, rmq)
	if err != nil {
		return k8sresource.MustParse("0"), err
	}

	for _, t := range sts.Spec.VolumeClaimTemplates {
		if t.Name == "persistence" {
			return t.Spec.Resources.Requests[corev1.ResourceStorage], nil
		}
	}
	return k8sresource.MustParse("0"), nil
}

// deleteSts deletes a sts without deleting pods and PVCs
// using DeletePropagationPolicy set to 'Orphan'
func (p PersistenceScaler) deleteSts(ctx context.Context, rmq rabbitmqv1beta1.RabbitmqCluster) error {
	logger := ctrl.LoggerFrom(ctx)
	logger.Info("deleting statefulSet (pods won't be deleted)", "statefulSet", rmq.ChildResourceName("server"))

	sts, err := p.getSts(ctx, rmq)
	if client.IgnoreNotFound(err) != nil {
		logErr := fmt.Errorf("failed to get statefulset from Kubernetes API: %w", err)
		logger.Error(logErr, "Could not read existing statefulset")
		return logErr
	}

	// The StatefulSet may have already been deleted. If so, there is no need to delete it again.
	if k8serrors.IsNotFound(err) {
		logger.Info("statefulset has already been deleted", "StatefulSet", rmq.Name, "RabbitmqCluster", rmq.Name)
		return nil
	}

	deletePropagationPolicy := metav1.DeletePropagationOrphan
	if err = p.Client.AppsV1().StatefulSets(sts.Namespace).Delete(ctx, sts.Name, metav1.DeleteOptions{PropagationPolicy: &deletePropagationPolicy}); err != nil {
		msg := "failed to delete statefulSet"
		logger.Error(err, msg, "statefulSet", sts.Name)
		return fmt.Errorf("%s %s: %w", msg, sts.Name, err)
	}

	if err := retryWithInterval(logger, "delete statefulSet", 10, 3*time.Second, func() bool {
		_, getErr := p.Client.AppsV1().StatefulSets(rmq.Namespace).Get(ctx, rmq.ChildResourceName("server"), metav1.GetOptions{})
		return k8serrors.IsNotFound(getErr)
	}); err != nil {
		msg := "statefulSet not deleting after 30 seconds"
		logger.Error(err, msg, "statefulSet", sts.Name)
		return fmt.Errorf("%s %s: %w", msg, sts.Name, err)
	}
	logger.Info("statefulSet deleted", "statefulSet", sts.Name)
	return nil
}

func (p PersistenceScaler) scaleUpPVCs(ctx context.Context, rmq rabbitmqv1beta1.RabbitmqCluster, pvcs []*corev1.PersistentVolumeClaim, desiredCapacity k8sresource.Quantity) error {
	logger := ctrl.LoggerFrom(ctx)

	for _, pvc := range pvcs {
		// To minimise any timing windows, retrieve the latest version of this PVC before updating
		pvc, err := p.Client.CoreV1().PersistentVolumeClaims(pvc.Namespace).Get(ctx, pvc.Name, metav1.GetOptions{})
		if err != nil {
			logErr := fmt.Errorf("failed to get PVC from Kubernetes API: %w", err)
			logger.Error(logErr, "Could not read existing PVC")
			return logErr
		}

		pvc.Spec.Resources.Requests[corev1.ResourceStorage] = desiredCapacity
		_, err = p.Client.CoreV1().PersistentVolumeClaims(rmq.Namespace).Update(ctx, pvc, metav1.UpdateOptions{})
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
		logger.V(1).Info("retrying again", "action", msg, "interval", interval, "attempt", i+1)
	}
	return fmt.Errorf("failed to %s after %d retries", msg, retry)
}
