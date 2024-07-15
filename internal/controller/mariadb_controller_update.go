package controller

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/go-logr/logr"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	labels "github.com/mariadb-operator/mariadb-operator/pkg/builder/labels"
	podpkg "github.com/mariadb-operator/mariadb-operator/pkg/pod"
	"github.com/mariadb-operator/mariadb-operator/pkg/wait"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	klabels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func shouldReconcileUpdates(mdb *mariadbv1alpha1.MariaDB) bool {
	if mdb.IsRestoringBackup() || mdb.IsResizingStorage() || mdb.IsSwitchingPrimary() || mdb.HasGaleraNotReadyCondition() {
		return false
	}
	if mdb.Spec.UpdateStrategy.Type != mariadbv1alpha1.ReplicasFirstPrimaryLast {
		return false
	}
	return true
}

func (r *MariaDBReconciler) reconcileUpdates(ctx context.Context, mdb *mariadbv1alpha1.MariaDB) (ctrl.Result, error) {
	if !shouldReconcileUpdates(mdb) {
		return ctrl.Result{}, nil
	}
	logger := log.FromContext(ctx).WithName("update")

	stsUpdateRevision, err := r.getStatefulSetRevision(ctx, mdb)
	if err != nil {
		return ctrl.Result{}, err
	}
	if stsUpdateRevision == "" {
		logger.V(1).Info("StatefulSet status.updateRevision not set. Requeuing...")
		return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
	}

	var podsByRole podRoleSet
	if result, err := r.getPodsByRole(ctx, mdb, &podsByRole, logger); !result.IsZero() || err != nil {
		return result, err
	}

	stalePodNames := podsByRole.getStalePodNames(stsUpdateRevision)
	if len(stalePodNames) == 0 {
		return ctrl.Result{}, nil
	}
	logger.V(1).Info("Detected stale Pods that need updating", "pods", stalePodNames)

	if result, err := r.waitForReadyStatus(ctx, mdb, logger); !result.IsZero() || err != nil {
		return result, err
	}

	for _, replicaPod := range podsByRole.replicas {
		if podpkg.PodUpdated(&replicaPod, stsUpdateRevision) {
			logger.V(1).Info("Replica Pod up to date", "pod", replicaPod.Name)
			continue
		}
		logger.Info("Updating replica Pod", "pod", replicaPod.Name)
		if err := r.updatePod(ctx, &replicaPod, stsUpdateRevision, logger); err != nil {
			return ctrl.Result{}, fmt.Errorf("error updating replica Pod '%s': %v", replicaPod.Name, err)
		}
		return ctrl.Result{Requeue: true}, nil
	}

	primaryPod := podsByRole.primary
	if podpkg.PodUpdated(&primaryPod, stsUpdateRevision) {
		logger.V(1).Info("Primary Pod up to date", "pod", primaryPod.Name)
		return ctrl.Result{}, nil
	}

	logger.Info("Updating primary Pod", "pod", primaryPod.Name)
	if err := r.updatePod(ctx, &primaryPod, stsUpdateRevision, logger); err != nil {
		return ctrl.Result{}, fmt.Errorf("error updating primary Pod '%s': %v", primaryPod.Name, err)
	}
	return ctrl.Result{}, nil
}

func (r *MariaDBReconciler) waitForReadyStatus(ctx context.Context, mdb *mariadbv1alpha1.MariaDB, logger logr.Logger) (ctrl.Result, error) {
	var sts appsv1.StatefulSet
	if err := r.Get(ctx, client.ObjectKeyFromObject(mdb), &sts); err != nil {
		return ctrl.Result{}, err
	}
	if sts.Status.ReadyReplicas != mdb.Spec.Replicas {
		logger.V(1).Info("Waiting for all Pods to be ready to proceed with the update. Requeuing...")
		return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
	}

	if mdb.IsMaxScaleEnabled() {
		mxs, err := r.RefResolver.MaxScale(ctx, mdb.Spec.MaxScaleRef, mdb.Namespace)
		if err != nil {
			return ctrl.Result{}, err
		}
		if !mxs.IsReady() {
			logger.V(1).Info("Waiting for MaxScale to be ready to proceed with the update. Requeuing...")
			return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
		}
	}
	return ctrl.Result{}, nil
}

func (r *MariaDBReconciler) updatePod(ctx context.Context, pod *corev1.Pod, updateRevision string, logger logr.Logger) error {
	if err := r.Delete(ctx, pod); err != nil {
		return fmt.Errorf("error deleting Pod '%s': %v", pod.Name, err)
	}

	updateCtx, cancel := context.WithTimeout(ctx, 1*time.Minute)
	defer cancel()
	if err := r.pollUntilPodUpdated(updateCtx, client.ObjectKeyFromObject(pod), updateRevision, logger); err != nil {
		return fmt.Errorf("error waiting for Pod '%s' to be updated: %v", pod.Name, err)
	}
	return nil
}

func (r *MariaDBReconciler) pollUntilPodUpdated(ctx context.Context, podKey types.NamespacedName, updateRevision string,
	logger logr.Logger) error {
	return wait.PollUntilSucessWithTimeout(ctx, logger, func(ctx context.Context) error {
		var pod corev1.Pod
		if err := r.Get(ctx, podKey, &pod); err != nil {
			return fmt.Errorf("error getting Pod '%s': %v", podKey.Name, err)
		}
		if podpkg.PodUpdated(&pod, updateRevision) {
			return nil
		}
		return errors.New("Pod stale")
	})
}

type podRoleSet struct {
	replicas []corev1.Pod
	primary  corev1.Pod
}

func (p *podRoleSet) getStalePodNames(updateRevision string) []string {
	var podNames []string
	for _, r := range p.replicas {
		if !podpkg.PodUpdated(&r, updateRevision) {
			podNames = append(podNames, r.Name)
		}
	}
	if !podpkg.PodUpdated(&p.primary, updateRevision) {
		podNames = append(podNames, p.primary.Name)
	}
	return podNames
}

func (r *MariaDBReconciler) getPodsByRole(ctx context.Context, mdb *mariadbv1alpha1.MariaDB, podsByRole *podRoleSet,
	logger logr.Logger) (ctrl.Result, error) {
	currentPrimary := ptr.Deref(mdb.Status.CurrentPrimary, "")
	if currentPrimary == "" {
		logger.V(1).Info("MariaDB status.currentPrimary not set. Requeuing...")
		return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
	}
	if mdb.Spec.Replicas == 0 {
		logger.V(1).Info("MariaDB is downscaled. Requeuing...")
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	list := corev1.PodList{}
	listOpts := &client.ListOptions{
		LabelSelector: klabels.SelectorFromSet(
			labels.NewLabelsBuilder().
				WithMariaDBSelectorLabels(mdb).
				Build(),
		),
		Namespace: mdb.GetNamespace(),
	}
	if err := r.List(ctx, &list, listOpts); err != nil {
		return ctrl.Result{}, fmt.Errorf("error listing Pods: %v", err)
	}

	numPods := len(list.Items)
	numReplicas := int(mdb.Spec.Replicas)
	if len(list.Items) != int(mdb.Spec.Replicas) {
		logger.V(1).Info("Number of Pods does not match MariaDB replicas. Requeuing...", "pods", numPods, "mariadb-replicas", numReplicas)
		return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
	}

	var replicas []corev1.Pod
	var primary *corev1.Pod
	for _, pod := range list.Items {
		if pod.Name == currentPrimary {
			primary = &pod
		} else {
			replicas = append(replicas, pod)
		}
	}
	if mdb.IsHAEnabled() && len(replicas) == 0 {
		return ctrl.Result{}, errors.New("no replica Pods found")
	}
	if primary == nil {
		return ctrl.Result{}, errors.New("primary Pod not found")
	}
	sort.Slice(replicas, func(i, j int) bool {
		return replicas[i].Name > replicas[j].Name
	})

	if podsByRole == nil {
		podsByRole = &podRoleSet{}
	}
	podsByRole.replicas = replicas
	podsByRole.primary = *primary

	return ctrl.Result{}, nil
}
