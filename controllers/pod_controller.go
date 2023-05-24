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
	mariadbpod "github.com/mariadb-operator/mariadb-operator/pkg/pod"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

var (
	errPodAnnotationNotFound = errors.New("MariaDB annotation not found in Pod")
)

type PodReadyReconciler interface {
	ReconcilePodReady(context.Context, corev1.Pod, *mariadbv1alpha1.MariaDB) error
	ReconcilePodNotReady(context.Context, corev1.Pod, *mariadbv1alpha1.MariaDB) error
}

// PodReconciler reconciles a Pod object
type PodReconciler struct {
	client.Client
	Annotation         string
	PodReadyReconciler PodReadyReconciler
}

//+kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *PodReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var pod corev1.Pod
	if err := r.Get(ctx, req.NamespacedName, &pod); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	mariadb, err := r.mariadbFromPod(ctx, pod)
	if err != nil {
		if errors.Is(err, errPodAnnotationNotFound) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if mariadbpod.PodReady(&pod) {
		if err := r.PodReadyReconciler.ReconcilePodReady(ctx, pod, mariadb); err != nil {
			return ctrl.Result{}, fmt.Errorf("error reconciling Pod '%s' in Ready state: %v", pod.Name, err)
		}
	} else {
		if err := r.PodReadyReconciler.ReconcilePodNotReady(ctx, pod, mariadb); err != nil {
			return ctrl.Result{}, fmt.Errorf("error reconciling Pod '%s' in non Ready state: %v", pod.Name, err)
		}
	}
	return ctrl.Result{}, nil
}

func (r *PodReconciler) mariadbFromPod(ctx context.Context, pod corev1.Pod) (*mariadbv1alpha1.MariaDB, error) {
	mariadbAnnotation, ok := pod.Annotations[annotation.PodMariadbAnnotation]
	if !ok {
		return nil, errPodAnnotationNotFound
	}

	var mariadb mariadbv1alpha1.MariaDB
	key := types.NamespacedName{
		Name:      mariadbAnnotation,
		Namespace: pod.Namespace,
	}
	if err := r.Get(ctx, key, &mariadb); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, err
		}
		return nil, fmt.Errorf("error getting MariaDB from Pod '%s': %v", pod.Name, err)
	}
	return &mariadb, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PodReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Pod{}).
		WithEventFilter(r.podPredicate()).
		Complete(r)
}

func (r *PodReconciler) podPredicate() predicate.Predicate {
	hasAnnotations := func(o client.Object) bool {
		annotations := o.GetAnnotations()
		if _, ok := annotations[annotation.PodMariadbAnnotation]; !ok {
			return false
		}
		if _, ok := annotations[r.Annotation]; !ok {
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
