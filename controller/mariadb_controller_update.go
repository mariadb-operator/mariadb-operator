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
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	klabels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func (r *MariaDBReconciler) reconcileUpdates(ctx context.Context, mdb *mariadbv1alpha1.MariaDB) (ctrl.Result, error) {
	if mdb.Spec.UpdateStrategy.Type != mariadbv1alpha1.ReplicasFirstPrimaryLast {
		return ctrl.Result{}, nil
	}
	logger := log.FromContext(ctx).WithName("update")

	var sts appsv1.StatefulSet
	if err := r.Get(ctx, client.ObjectKeyFromObject(mdb), &sts); err != nil {
		return ctrl.Result{}, err
	}
	stsUpdateRevision := sts.Status.UpdateRevision
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

	if sts.Status.ReadyReplicas != mdb.Spec.Replicas {
		logger.V(1).Info("Waiting for all Pods to be ready to proceed with the update. Requeuing...")
		return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
	}
	return ctrl.Result{}, nil
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
	if len(replicas) == 0 {
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
