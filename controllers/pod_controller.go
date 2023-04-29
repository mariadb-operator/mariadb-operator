/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"errors"
	"fmt"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/pkg/annotation"
	labels "github.com/mariadb-operator/mariadb-operator/pkg/builder/labels"
	"github.com/mariadb-operator/mariadb-operator/pkg/controller/replication"
	mariadbclient "github.com/mariadb-operator/mariadb-operator/pkg/mariadb"
	"github.com/mariadb-operator/mariadb-operator/pkg/pod"
	mariadbpod "github.com/mariadb-operator/mariadb-operator/pkg/pod"
	"github.com/mariadb-operator/mariadb-operator/pkg/refresolver"
	"github.com/mariadb-operator/mariadb-operator/pkg/statefulset"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// PodReconciler reconciles a Pod object
type PodReconciler struct {
	client.Client
	Scheme      *runtime.Scheme
	RefResolver *refresolver.RefResolver
}

//+kubebuilder:rbac:groups=mariadb.mmontes.io,resources=pods,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=mariadb.mmontes.io,resources=pods/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=mariadb.mmontes.io,resources=pods/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *PodReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var pod corev1.Pod
	if err := r.Get(ctx, req.NamespacedName, &pod); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	mariadb, err := r.mariadbFromPod(ctx, pod)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error getting MariaDB from Pod annotation: %v", err)
	}
	if mariadb.Spec.Replication == nil || mariadb.Status.CurrentPrimaryPodIndex == nil {
		return ctrl.Result{}, nil
	}

	if mariadbpod.PodReady(&pod) {
		if err := r.reconcilePodReady(ctx, pod, mariadb); err != nil {
			return ctrl.Result{}, fmt.Errorf("error reconciling Pod '%s' in Ready state: %v", pod.Name, err)
		}
	} else {
		if err := r.reconcilePodNotReady(ctx, pod, mariadb); err != nil {
			return ctrl.Result{}, fmt.Errorf("error reconciling Pod '%s' in non Ready state: %v", pod.Name, err)
		}
	}
	return ctrl.Result{}, nil
}

func (r *PodReconciler) mariadbFromPod(ctx context.Context, pod corev1.Pod) (*mariadbv1alpha1.MariaDB, error) {
	mariadbAnnotation, ok := pod.Annotations[annotation.PodMariadbAnnotation]
	if !ok {
		return nil, fmt.Errorf("MariaDB annotation not found in Pod '%s'", pod.Name)
	}
	log.FromContext(ctx).V(1).Info("Reconciling Pod in Ready state", "pod", pod.Name)

	var mariadb mariadbv1alpha1.MariaDB
	key := types.NamespacedName{
		Name:      mariadbAnnotation,
		Namespace: pod.Namespace,
	}
	if err := r.Get(ctx, key, &mariadb); err != nil {
		return nil, fmt.Errorf("error getting MariaDB from Pod '%s': %v", pod.Name, err)
	}
	return &mariadb, nil
}

func (r *PodReconciler) reconcilePodReady(ctx context.Context, pod corev1.Pod, mariadb *mariadbv1alpha1.MariaDB) error {
	if mariadb.IsSwitchingPrimary() {
		return nil
	}
	log.FromContext(ctx).V(1).Info("Reconciling Pod in Ready state", "pod", pod.Name)

	index, err := statefulset.PodIndex(pod.Name)
	if err != nil {
		return fmt.Errorf("error getting Pod index: %v", err)
	}

	client, err := mariadbclient.NewRootClientWithPodIndex(ctx, mariadb, r.RefResolver, *index)
	if err != nil {
		return fmt.Errorf("error connecting to replica '%d': %v", *index, err)
	}
	defer client.Close()

	config := replication.NewReplicationConfig(mariadb, client, r.Client)

	if *index == *mariadb.Status.CurrentPrimaryPodIndex {
		if err := config.ConfigurePrimary(ctx, *index); err != nil {
			return fmt.Errorf("error configuring primary in replica '%d': %v", *index, err)
		}
		return nil
	}
	if err := config.ConfigureReplica(ctx, *index, *mariadb.Status.CurrentPrimaryPodIndex); err != nil {
		return fmt.Errorf("error configuring replication in replica '%d': %v", *index, err)
	}
	return nil
}

func (r *PodReconciler) reconcilePodNotReady(ctx context.Context, pod corev1.Pod, mariadb *mariadbv1alpha1.MariaDB) error {
	if mariadb.IsSwitchingPrimary() {
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

	if err := r.patch(ctx, mariadb, func(mdb *mariadbv1alpha1.MariaDB) {
		mdb.Spec.Replication.Primary.PodIndex = *healthyIndex
	}); err != nil {
		return fmt.Errorf("error updating primary: %v", err)
	}
	return nil
}

func (r *PodReconciler) healthyReplica(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) (*int, error) {
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

func (r *PodReconciler) patch(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	patcher func(*mariadbv1alpha1.MariaDB)) error {
	patch := client.MergeFrom(mariadb.DeepCopy())
	patcher(mariadb)

	if err := r.Patch(ctx, mariadb, patch); err != nil {
		return fmt.Errorf("error patching MariaDB: %v", err)
	}
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PodReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Pod{}).
		WithEventFilter(mariadbPodsPredicate()).
		Complete(r)
}

func mariadbPodsPredicate() predicate.Predicate {
	hasAnnotations := func(o client.Object) bool {
		annotations := o.GetAnnotations()
		if _, ok := annotations[annotation.PodReplicationAnnotation]; !ok {
			return false
		}
		if _, ok := annotations[annotation.PodMariadbAnnotation]; !ok {
			return false
		}
		return true
	}
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return hasAnnotations(e.Object)
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return false
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			if !hasAnnotations(e.ObjectOld) || !hasAnnotations(e.ObjectNew) {
				return false
			}
			oldPod, ok := e.ObjectOld.(*corev1.Pod)
			if !ok {
				return false
			}
			newPod, ok := e.ObjectNew.(*corev1.Pod)
			if !ok {
				return false
			}
			return mariadbpod.PodReady(oldPod) != mariadbpod.PodReady(newPod)
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return hasAnnotations(e.Object)
		},
	}
}
