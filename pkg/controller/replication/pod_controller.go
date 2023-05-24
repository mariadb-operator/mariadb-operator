package replication

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/go-multierror"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/pkg/builder"
	labels "github.com/mariadb-operator/mariadb-operator/pkg/builder/labels"
	"github.com/mariadb-operator/mariadb-operator/pkg/conditions"
	"github.com/mariadb-operator/mariadb-operator/pkg/controller/secret"
	"github.com/mariadb-operator/mariadb-operator/pkg/pod"
	"github.com/mariadb-operator/mariadb-operator/pkg/refresolver"
	"github.com/mariadb-operator/mariadb-operator/pkg/statefulset"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type PodReplicationReconciler struct {
	client.Client
	replConfig       *ReplicationConfig
	secretReconciler *secret.SecretReconciler
	builder          *builder.Builder
	refResolver      *refresolver.RefResolver
}

func NewPodReplicationReconciler(client client.Client, replConfig *ReplicationConfig,
	secretReconciler *secret.SecretReconciler, builder *builder.Builder) *PodReplicationReconciler {
	return &PodReplicationReconciler{
		Client:           client,
		replConfig:       replConfig,
		secretReconciler: secretReconciler,
		builder:          builder,
		refResolver:      refresolver.New(client),
	}
}

func (r *PodReplicationReconciler) ReconcilePodReady(ctx context.Context, pod corev1.Pod, mariadb *mariadbv1alpha1.MariaDB) error {
	if !r.shouldReconcile(mariadb) {
		return nil
	}
	log.FromContext(ctx).V(1).Info("Reconciling Pod in Ready state", "pod", pod.Name)

	index, err := statefulset.PodIndex(pod.Name)
	if err != nil {
		return fmt.Errorf("error getting Pod index: %v", err)
	}

	if *index != *mariadb.Status.CurrentPrimaryPodIndex {
		return nil
	}
	healthyIndex, err := r.healthyReplica(ctx, mariadb)
	if err != nil {
		return fmt.Errorf("error getting healthy replica: %v", err)
	}

	var errBundle *multierror.Error
	err = r.patch(ctx, mariadb, func(mdb *mariadbv1alpha1.MariaDB) {
		mdb.Spec.Replication.Primary.PodIndex = *healthyIndex
	})
	errBundle = multierror.Append(errBundle, err)

	err = r.patchStatus(ctx, mariadb, func(status *mariadbv1alpha1.MariaDBStatus) {
		conditions.SetPrimarySwitching(status, mariadb)
	})
	errBundle = multierror.Append(errBundle, err)

	if err := errBundle.ErrorOrNil(); err != nil {
		return fmt.Errorf("error patching MariaDB: %v", err)
	}
	return nil
}

func (r *PodReplicationReconciler) ReconcilePodNotReady(ctx context.Context, pod corev1.Pod, mariadb *mariadbv1alpha1.MariaDB) error {
	if !r.shouldReconcile(mariadb) {
		return nil
	}
	log.FromContext(ctx).V(1).Info("Reconciling Pod in non Ready state", "pod", pod.Name)

	index, err := statefulset.PodIndex(pod.Name)
	if err != nil {
		return fmt.Errorf("error getting Pod index: %v", err)
	}

	if *index != *mariadb.Status.CurrentPrimaryPodIndex {
		return nil
	}
	healthyIndex, err := r.healthyReplica(ctx, mariadb)
	if err != nil {
		return fmt.Errorf("error getting healthy replica: %v", err)
	}

	var errBundle *multierror.Error
	err = r.patch(ctx, mariadb, func(mdb *mariadbv1alpha1.MariaDB) {
		mdb.Spec.Replication.Primary.PodIndex = *healthyIndex
	})
	errBundle = multierror.Append(errBundle, err)

	err = r.patchStatus(ctx, mariadb, func(status *mariadbv1alpha1.MariaDBStatus) {
		conditions.SetPrimarySwitching(status, mariadb)
	})
	errBundle = multierror.Append(errBundle, err)

	if err := errBundle.ErrorOrNil(); err != nil {
		return fmt.Errorf("error patching MariaDB: %v", err)
	}
	return nil
}

func (r *PodReplicationReconciler) shouldReconcile(mariadb *mariadbv1alpha1.MariaDB) bool {
	if mariadb.IsConfiguringReplication() || mariadb.IsRestoringBackup() {
		return false
	}
	if mariadb.Spec.Replication == nil || mariadb.Status.CurrentPrimaryPodIndex == nil {
		return false
	}
	if !mariadb.Spec.Replication.Primary.AutomaticFailover {
		return false
	}
	return true
}

func (r *PodReplicationReconciler) healthyReplica(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) (*int, error) {
	podLabels :=
		labels.NewLabelsBuilder().
			WithMariaDB(mariadb).
			Build()
	podList := corev1.PodList{}
	if err := r.List(ctx, &podList, client.MatchingLabels(podLabels)); err != nil {
		return nil, fmt.Errorf("error listing Pods: %v", err)
	}
	for _, p := range podList.Items {
		index, err := statefulset.PodIndex(p.Name)
		if err != nil {
			return nil, fmt.Errorf("error getting index for Pod '%s': %v", p.Name, err)
		}
		if *index == *mariadb.Status.CurrentPrimaryPodIndex {
			continue
		}
		if pod.PodReady(&p) {
			return index, nil
		}
	}
	return nil, errors.New("no healthy replicas available")
}

func (r *PodReplicationReconciler) patch(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	patcher func(*mariadbv1alpha1.MariaDB)) error {
	patch := client.MergeFrom(mariadb.DeepCopy())
	patcher(mariadb)

	if err := r.Patch(ctx, mariadb, patch); err != nil {
		return fmt.Errorf("error patching MariaDB: %v", err)
	}
	return nil
}

func (r *PodReplicationReconciler) patchStatus(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	patcher func(*mariadbv1alpha1.MariaDBStatus)) error {
	patch := client.MergeFrom(mariadb.DeepCopy())
	patcher(&mariadb.Status)

	if err := r.Client.Status().Patch(ctx, mariadb, patch); err != nil {
		return fmt.Errorf("error patching MariaDB status: %v", err)
	}
	return nil
}
