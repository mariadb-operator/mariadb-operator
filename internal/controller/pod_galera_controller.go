package controller

import (
	"context"
	"fmt"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/pkg/health"
	mdbpod "github.com/mariadb-operator/mariadb-operator/pkg/pod"
	"github.com/mariadb-operator/mariadb-operator/pkg/statefulset"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// PodGaleraController reconciles a Pod object
type PodGaleraController struct {
	client.Client
	recorder record.EventRecorder
}

func NewPodGaleraController(client client.Client, recorder record.EventRecorder) PodReadinessController {
	return &PodGaleraController{
		Client:   client,
		recorder: recorder,
	}
}

func (r *PodGaleraController) ReconcilePodReady(ctx context.Context, pod corev1.Pod, mariadb *mariadbv1alpha1.MariaDB) error {
	if !r.shouldReconcile(mariadb) {
		return nil
	}
	logger := log.FromContext(ctx)
	if mariadb.Status.CurrentPrimaryPodIndex == nil {
		logger.V(1).Info("'status.currentPrimaryPodIndex' must be set. Skipping")
		return nil
	}
	logger.V(1).Info("Reconciling Pod in Ready state", "pod", pod.Name)

	currentPrimaryPodKey := types.NamespacedName{
		Name:      statefulset.PodName(mariadb.ObjectMeta, *mariadb.Status.CurrentPrimaryPodIndex),
		Namespace: mariadb.Namespace,
	}
	var currentPrimaryPod corev1.Pod
	if err := r.Get(ctx, currentPrimaryPodKey, &currentPrimaryPod); err != nil {
		return fmt.Errorf("error getting current primary Pod: %v", err)
	}
	if mdbpod.PodReady(&currentPrimaryPod) {
		return nil
	}

	fromIndex := mariadb.Status.CurrentPrimaryPodIndex
	toIndex, err := statefulset.PodIndex(pod.Name)
	if err != nil {
		return fmt.Errorf("error getting Pod index: %v", err)
	}
	if *fromIndex == *toIndex {
		return nil
	}

	logger.Info("Switching primary", "from-index", *fromIndex, "to-index", *toIndex)
	if err := r.patch(ctx, mariadb, func(mdb *mariadbv1alpha1.MariaDB) {
		mariadb.Spec.Galera.Primary.PodIndex = toIndex
	}); err != nil {
		return err
	}

	logger.Info("Switching primary", "from-index", *fromIndex, "to-index", *toIndex)
	r.recorder.Eventf(mariadb, corev1.EventTypeNormal, mariadbv1alpha1.ReasonPrimarySwitching,
		"Switching primary from index '%d' to index '%d'", *fromIndex, *toIndex)

	return nil
}

func (r *PodGaleraController) ReconcilePodNotReady(ctx context.Context, pod corev1.Pod, mariadb *mariadbv1alpha1.MariaDB) error {
	if !r.shouldReconcile(mariadb) {
		return nil
	}
	logger := log.FromContext(ctx).WithName("pod-galera")
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

	if err := r.patch(ctx, mariadb, func(mdb *mariadbv1alpha1.MariaDB) {
		mdb.Spec.Galera.Primary.PodIndex = toIndex
	}); err != nil {
		return err
	}

	logger.Info("Switching primary", "from-index", *fromIndex, "to-index", *toIndex)
	r.recorder.Eventf(mariadb, corev1.EventTypeNormal, mariadbv1alpha1.ReasonPrimarySwitching,
		"Switching primary from index '%d' to index '%d'", *fromIndex, *toIndex)

	return nil
}

func (r *PodGaleraController) shouldReconcile(mariadb *mariadbv1alpha1.MariaDB) bool {
	if !mariadb.IsGaleraEnabled() || mariadb.IsMaxScaleEnabled() || mariadb.IsRestoringBackup() {
		return false
	}
	primaryGalera := ptr.Deref(mariadb.Spec.Galera, mariadbv1alpha1.Galera{}).Primary
	automaticFailover := ptr.Deref(primaryGalera.AutomaticFailover, false)

	return automaticFailover && mariadb.HasGaleraConfiguredCondition() && mariadb.HasGaleraReadyCondition()
}

func (r *PodGaleraController) patch(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	patcher func(*mariadbv1alpha1.MariaDB)) error {
	patch := client.MergeFrom(mariadb.DeepCopy())
	patcher(mariadb)

	if err := r.Patch(ctx, mariadb, patch); err != nil {
		return fmt.Errorf("error patching MariaDB: %v", err)
	}
	return nil
}
