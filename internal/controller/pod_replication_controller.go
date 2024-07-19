package controller

import (
	"context"
	"fmt"

	"github.com/hashicorp/go-multierror"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/pkg/builder"
	condition "github.com/mariadb-operator/mariadb-operator/pkg/condition"
	"github.com/mariadb-operator/mariadb-operator/pkg/controller/replication"
	"github.com/mariadb-operator/mariadb-operator/pkg/health"
	"github.com/mariadb-operator/mariadb-operator/pkg/refresolver"
	"github.com/mariadb-operator/mariadb-operator/pkg/statefulset"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// PodReplicationController reconciles a Pod object
type PodReplicationController struct {
	client.Client
	recorder    record.EventRecorder
	builder     *builder.Builder
	refResolver *refresolver.RefResolver
	replConfig  *replication.ReplicationConfig
}

func NewPodReplicationController(client client.Client, recorder record.EventRecorder, builder *builder.Builder,
	refResolver *refresolver.RefResolver, replConfig *replication.ReplicationConfig) PodReadinessController {
	return &PodReplicationController{
		Client:      client,
		recorder:    recorder,
		builder:     builder,
		refResolver: refResolver,
		replConfig:  replConfig,
	}
}

func (r *PodReplicationController) ReconcilePodReady(ctx context.Context, pod corev1.Pod, mariadb *mariadbv1alpha1.MariaDB) error {
	return nil
}

func (r *PodReplicationController) ReconcilePodNotReady(ctx context.Context, pod corev1.Pod, mariadb *mariadbv1alpha1.MariaDB) error {
	if !shouldReconcile(mariadb) {
		return nil
	}
	logger := log.FromContext(ctx).WithName("pod-replication")
	if mariadb.Status.CurrentPrimaryPodIndex == nil {
		logger.V(1).Info("'status.currentPrimaryPodIndex' must be set. Skipping")
		return nil
	}
	logger.V(1).Info("Reconciling Pod in non Ready state", "pod", pod.Name)

	index, err := statefulset.PodIndex(pod.Name)
	if err != nil {
		return fmt.Errorf("error getting Pod index: %v", err)
	}
	if *index != *mariadb.Status.CurrentPrimaryPodIndex {
		return nil
	}

	fromIndex := mariadb.Status.CurrentPrimaryPodIndex
	toIndex, err := health.HealthyMariaDBReplica(ctx, r, mariadb)
	if err != nil {
		return fmt.Errorf("error getting healthy replica: %v", err)
	}

	var errBundle *multierror.Error
	err = r.patch(ctx, mariadb, func(mdb *mariadbv1alpha1.MariaDB) {
		mdb.Replication().Primary.PodIndex = toIndex
	})
	errBundle = multierror.Append(errBundle, err)

	err = r.patchStatus(ctx, mariadb, func(status *mariadbv1alpha1.MariaDBStatus) {
		condition.SetPrimarySwitching(status, mariadb)
	})
	errBundle = multierror.Append(errBundle, err)

	if err := errBundle.ErrorOrNil(); err != nil {
		return fmt.Errorf("error patching MariaDB: %v", err)
	}

	logger.Info("Switching primary", "from-index", fromIndex, "to-index", *toIndex)
	r.recorder.Eventf(mariadb, corev1.EventTypeNormal, mariadbv1alpha1.ReasonPrimarySwitching,
		"Switching primary from index '%d' to index '%d'", *fromIndex, *toIndex)

	return nil
}

func shouldReconcile(mariadb *mariadbv1alpha1.MariaDB) bool {
	if mariadb.IsMaxScaleEnabled() || mariadb.IsRestoringBackup() {
		return false
	}
	primaryRepl := ptr.Deref(mariadb.Replication().Primary, mariadbv1alpha1.PrimaryReplication{})
	return mariadb.Replication().Enabled && *primaryRepl.AutomaticFailover && mariadb.IsReplicationConfigured()
}

func (r *PodReplicationController) patch(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	patcher func(*mariadbv1alpha1.MariaDB)) error {
	patch := client.MergeFrom(mariadb.DeepCopy())
	patcher(mariadb)

	if err := r.Patch(ctx, mariadb, patch); err != nil {
		return fmt.Errorf("error patching MariaDB: %v", err)
	}
	return nil
}

func (r *PodReplicationController) patchStatus(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	patcher func(*mariadbv1alpha1.MariaDBStatus)) error {
	patch := client.MergeFrom(mariadb.DeepCopy())
	patcher(&mariadb.Status)

	if err := r.Client.Status().Patch(ctx, mariadb, patch); err != nil {
		return fmt.Errorf("error patching MariaDB status: %v", err)
	}
	return nil
}
