package galera

import (
	"context"
	"errors"
	"fmt"
	"time"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	mariadbclient "github.com/mariadb-operator/mariadb-operator/pkg/client"
	"github.com/mariadb-operator/mariadb-operator/pkg/conditions"
	"github.com/mariadb-operator/mariadb-operator/pkg/pod"
	"github.com/mariadb-operator/mariadb-operator/pkg/refresolver"
	"github.com/mariadb-operator/mariadb-operator/pkg/statefulset"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
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
	if !mariadb.HasGaleraConfiguredCondition() || mariadb.HasGaleraNotReadyCondition() {
		return nil
	}
	healthyCtx, cancel := context.WithTimeout(ctx, mariadb.Spec.Galera.Recovery.HealthyTimeoutOrDefault())
	defer cancel()

	logger := log.FromContext(ctx)
	logger.Info("Checking Galera cluster health")
	healthy, err := r.pollUntilHealthy(healthyCtx, mariadb)
	if err != nil {
		return err
	}

	if healthy {
		logger.Info("Galera cluster is healthy")
		return nil
	}
	logger.Info("Galera cluster is not healthy")
	return r.patchStatus(ctx, mariadb, func(status *mariadbv1alpha1.MariaDBStatus) {
		conditions.SetGaleraNotReady(status, mariadb)
	})
}

func (r *PodGaleraReconciler) pollUntilHealthy(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) (bool, error) {
	// TODO: bump apimachinery and migrate to PollUntilContextTimeout.
	// See: https://pkg.go.dev/k8s.io/apimachinery@v0.27.2/pkg/util/wait#PollUntilContextTimeout
	err := wait.PollImmediateUntilWithContext(ctx, 1*time.Second, func(context.Context) (bool, error) {
		return r.isHealthy(ctx, mariadb)
	})
	if err != nil {
		if errors.Is(err, wait.ErrWaitTimeout) {
			return false, nil
		}
		return false, fmt.Errorf("error polling health: %v", err)
	}
	return true, nil
}

func (r *PodGaleraReconciler) isHealthy(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) (bool, error) {
	sts, err := r.statefulSet(ctx, mariadb)
	if err != nil {
		return false, fmt.Errorf("error getting StatefulSet: %v", err)
	}
	logger := log.FromContext(ctx)
	logger.V(1).Info("StatefulSet ready replicas", "replicas", sts.Status.ReadyReplicas)
	if sts.Status.ReadyReplicas == mariadb.Spec.Replicas {
		return true, nil
	}
	if sts.Status.ReadyReplicas == 0 {
		return false, nil
	}

	clientSet := mariadbclient.NewClientSet(mariadb, r.refResolver)
	defer clientSet.Close()
	client, err := r.readyClient(ctx, mariadb, clientSet)
	if err != nil {
		return false, fmt.Errorf("error getting ready client: %v", err)
	}

	status, err := client.GaleraClusterStatus(ctx)
	if err != nil {
		return false, fmt.Errorf("error getting Galera cluster status: %v", err)
	}
	logger.V(1).Info("Galera cluster status", "status", status)
	if status != "Primary" {
		return false, nil
	}

	size, err := client.GaleraClusterSize(ctx)
	if err != nil {
		return false, fmt.Errorf("error getting Galera cluster size: %v", err)
	}
	logger.V(1).Info("Galera cluster size", "size", size)
	if size != int(mariadb.Spec.Replicas) {
		return false, nil
	}

	return true, nil
}

func (r *PodGaleraReconciler) statefulSet(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) (*appsv1.StatefulSet, error) {
	var sts appsv1.StatefulSet
	if err := r.Get(ctx, client.ObjectKeyFromObject(mariadb), &sts); err != nil {
		return nil, err
	}
	return &sts, nil
}

func (r *PodGaleraReconciler) readyClient(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	clientSet *mariadbclient.ClientSet) (*mariadbclient.Client, error) {
	for i := 0; i < int(mariadb.Spec.Replicas); i++ {
		key := types.NamespacedName{
			Name:      statefulset.PodName(mariadb.ObjectMeta, i),
			Namespace: mariadb.Namespace,
		}
		var p corev1.Pod
		if err := r.Get(ctx, key, &p); err != nil {
			return nil, fmt.Errorf("error getting Pod: %v", err)
		}
		if !pod.PodReady(&p) {
			continue
		}

		if client, err := clientSet.ClientForIndex(ctx, i); err == nil {
			return client, nil
		}
	}
	return nil, errors.New("no Ready Pods were found")
}

func (r *PodGaleraReconciler) patchStatus(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	patcher func(*mariadbv1alpha1.MariaDBStatus)) error {
	patch := client.MergeFrom(mariadb.DeepCopy())
	patcher(&mariadb.Status)
	return r.Status().Patch(ctx, mariadb, patch)
}
