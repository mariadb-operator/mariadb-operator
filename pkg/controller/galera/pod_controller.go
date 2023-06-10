package galera

import (
	"context"
	"fmt"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	mariadbclient "github.com/mariadb-operator/mariadb-operator/pkg/client"
	"github.com/mariadb-operator/mariadb-operator/pkg/conditions"
	"github.com/mariadb-operator/mariadb-operator/pkg/refresolver"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type PodGaleraReconciler struct {
	client.Client
	refResolver *refresolver.RefResolver
}

func NewPodGaleraReconciler(client client.Client) *PodGaleraReconciler {
	return &PodGaleraReconciler{
		Client:      client,
		refResolver: refresolver.New(client),
	}
}

func (r *PodGaleraReconciler) ReconcilePodReady(ctx context.Context, pod corev1.Pod, mariadb *mariadbv1alpha1.MariaDB) error {
	return nil
}

func (r *PodGaleraReconciler) ReconcilePodNotReady(ctx context.Context, pod corev1.Pod, mariadb *mariadbv1alpha1.MariaDB) error {
	if !mariadb.HasGaleraConfiguredCondition() {
		return nil
	}
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

	// TODO:
	// If this happens consistently during 60s (configurable by user via spec.galera.galeraNotReadyTimeout) => not healthy:
	// - All Pods not Ready
	//	OR
	// - Some Pods not Ready, perform the following checks in the Ready ones:
	//	- SHOW STATUS LIKE 'wsrep_cluster_status':
	//		If the value of wsrep_cluster_status is "Primary" or "Non-Primary,"  it indicates that the cluster is healthy.
	//		If it shows "Disconnected" or "Non-Primary" for an extended period, there might be an issue.
	//		If any node is consistently in a different state or experiencing connection issues, it might indicate a problem.
	//	- SHOW STATUS LIKE 'wsrep_cluster_size';
	//		Ensure that the reported cluster size matches the number of nodes you expect.
	//		If the cluster size decreases unexpectedly, it could indicate a node failure or network issue.
	//	Maybe useful for recovery?
	//	- SHOW STATUS LIKE 'wsrep_local_state_comment';
	//		The output will display the state of each node, such as "Synced," "Donor," "Joining," "Joined," or "Disconnected."
	//		All nodes should ideally be in the "Synced" state.

	sts, err := r.statefulSet(ctx, mariadb)
	if err != nil {
		return false, fmt.Errorf("error getting StatefulSet: %v", err)
	}
	if sts.Status.ReadyReplicas == mariadb.Spec.Replicas {
		return true, nil
	}
	if sts.Status.ReadyReplicas == 0 {
		return false, nil
	}

	clientSet := mariadbclient.NewClientSet(mariadb, r.refResolver)
	defer clientSet.Close()

	return true, nil
}

func (r *PodGaleraReconciler) statefulSet(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) (*appsv1.StatefulSet, error) {
	var sts appsv1.StatefulSet
	if err := r.Get(ctx, client.ObjectKeyFromObject(mariadb), &sts); err != nil {
		return nil, err
	}
	return &sts, nil
}

func (r *PodGaleraReconciler) patchStatus(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	patcher func(*mariadbv1alpha1.MariaDBStatus)) error {
	patch := client.MergeFrom(mariadb.DeepCopy())
	patcher(&mariadb.Status)
	return r.Status().Patch(ctx, mariadb, patch)
}
