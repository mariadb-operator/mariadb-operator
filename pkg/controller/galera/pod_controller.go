package galera

import (
	"context"
	"fmt"
	"time"

	agentclient "github.com/mariadb-operator/agent/pkg/client"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/pkg/conditions"
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
	return nil
}

func (r *PodGaleraReconciler) ReconcilePodNotReady(ctx context.Context, pod corev1.Pod, mariadb *mariadbv1alpha1.MariaDB) error {
	healthy, err := r.IsGaleraHealthy(ctx, pod, mariadb)
	if err != nil {
		return err
	}
	if healthy {
		return nil
	}
	err = r.patchStatus(ctx, mariadb, func(status *mariadbv1alpha1.MariaDBStatus) {
		conditions.SetGaleraNotReady(status, mariadb)
	})
	return err
}

func (r *PodGaleraReconciler) IsGaleraHealthy(ctx context.Context, pod corev1.Pod, mariadb *mariadbv1alpha1.MariaDB) (bool, error) {
	log.FromContext(ctx).V(1).Info("Getting Galera state", "pod", pod.Name)

	agentClient, err := newAgentClient(mariadb, pod, agentclient.WithTimeout(1*time.Second))
	if err != nil {
		return false, fmt.Errorf("error getting agent client: %v", err)
	}

	state, err := agentClient.GaleraState.Get(ctx)
	if err != nil {
		if agentclient.IsNotFound(err) {
			return true, nil
		}
		return false, fmt.Errorf("error getting galera state: %v", err)
	}
	if !state.SafeToBootstrap {
		return false, nil
	}
	return true, nil
}

func (r *PodGaleraReconciler) patchStatus(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	patcher func(*mariadbv1alpha1.MariaDBStatus)) error {
	patch := client.MergeFrom(mariadb.DeepCopy())
	patcher(&mariadb.Status)
	return r.Status().Patch(ctx, mariadb, patch)
}
