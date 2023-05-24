package galera

import (
	"context"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type PodGaleraReconciler struct {
	client.Client
}

func NewPodGaleraReconciler(client client.Client) *PodGaleraReconciler {
	return &PodGaleraReconciler{
		Client: client,
	}
}

func (r *PodGaleraReconciler) ReconcilePodReady(ctx context.Context, pod corev1.Pod, mariadb *mariadbv1alpha1.MariaDB) error {
	if !r.shouldReconcile(mariadb) {
		return nil
	}
	log.FromContext(ctx).V(1).Info("Reconciling Pod in Ready state", "pod", pod.Name)

	return nil
}

func (r *PodGaleraReconciler) ReconcilePodNotReady(ctx context.Context, pod corev1.Pod, mariadb *mariadbv1alpha1.MariaDB) error {
	if !r.shouldReconcile(mariadb) {
		return nil
	}
	log.FromContext(ctx).V(1).Info("Reconciling Pod in non Ready state", "pod", pod.Name)

	return nil
}

func (r *PodGaleraReconciler) shouldReconcile(mariadb *mariadbv1alpha1.MariaDB) bool {
	if mariadb.IsRestoringBackup() || mariadb.Spec.Galera == nil {
		return false
	}
	return true
}
