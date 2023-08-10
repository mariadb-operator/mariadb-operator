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

	"github.com/hashicorp/go-multierror"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/pkg/annotation"
	"github.com/mariadb-operator/mariadb-operator/pkg/builder"
	labels "github.com/mariadb-operator/mariadb-operator/pkg/builder/labels"
	mariadbclient "github.com/mariadb-operator/mariadb-operator/pkg/client"
	"github.com/mariadb-operator/mariadb-operator/pkg/conditions"
	"github.com/mariadb-operator/mariadb-operator/pkg/controller/replication"
	"github.com/mariadb-operator/mariadb-operator/pkg/controller/secret"
	"github.com/mariadb-operator/mariadb-operator/pkg/pod"
	mariadbpod "github.com/mariadb-operator/mariadb-operator/pkg/pod"
	"github.com/mariadb-operator/mariadb-operator/pkg/predicate"
	"github.com/mariadb-operator/mariadb-operator/pkg/refresolver"
	"github.com/mariadb-operator/mariadb-operator/pkg/statefulset"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// PodReplicationController reconciles a Pod object
type PodReplicationController struct {
	client.Client
	Scheme           *runtime.Scheme
	ReplConfig       *replication.ReplicationConfig
	SecretReconciler *secret.SecretReconciler
	Builder          *builder.Builder
	RefResolver      *refresolver.RefResolver
}

//+kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *PodReplicationController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var pod corev1.Pod
	if err := r.Get(ctx, req.NamespacedName, &pod); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	mariadb, err := r.RefResolver.MariaDBFromAnnotation(ctx, pod.ObjectMeta)
	if err != nil {
		if errors.Is(err, refresolver.ErrMariaDBAnnotationNotFound) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	if !mariadb.Replication().Enabled || mariadb.Status.CurrentPrimaryPodIndex == nil ||
		mariadb.IsConfiguringReplication() || mariadb.IsRestoringBackup() {
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

func (r *PodReplicationController) reconcilePodReady(ctx context.Context, pod corev1.Pod, mariadb *mariadbv1alpha1.MariaDB) error {
	log.FromContext(ctx).V(1).Info("Reconciling Pod in Ready state", "pod", pod.Name)

	index, err := statefulset.PodIndex(pod.Name)
	if err != nil {
		return fmt.Errorf("error getting Pod index: %v", err)
	}

	client, err := mariadbclient.NewInternalClientWithPodIndex(ctx, mariadb, r.RefResolver, *index)
	if err != nil {
		return fmt.Errorf("error connecting to replica '%d': %v", *index, err)
	}
	defer client.Close()

	if *index == *mariadb.Status.CurrentPrimaryPodIndex {
		if err := r.ReplConfig.ConfigurePrimary(ctx, mariadb, client, *index); err != nil {
			return fmt.Errorf("error configuring primary in replica '%d': %v", *index, err)
		}
		return nil
	}
	if err := r.ReplConfig.ConfigureReplica(ctx, mariadb, client, *index, *mariadb.Status.CurrentPrimaryPodIndex); err != nil {
		return fmt.Errorf("error configuring replication in replica '%d': %v", *index, err)
	}
	return nil
}

func (r *PodReplicationController) reconcilePodNotReady(ctx context.Context, pod corev1.Pod, mariadb *mariadbv1alpha1.MariaDB) error {
	if !*mariadb.Replication().Primary.AutomaticFailover {
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
		mdb.Spec.Replication.Primary.PodIndex = healthyIndex
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

func (r *PodReplicationController) healthyReplica(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) (*int, error) {
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

// SetupWithManager sets up the controller with the Manager.
func (r *PodReplicationController) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Pod{}).
		WithEventFilter(
			predicate.PredicateChangedWithAnnotations(
				[]string{
					annotation.MariadbAnnotation,
					annotation.ReplicationAnnotation,
				},
				podHasChanged,
			),
		).
		Complete(r)
}

func podHasChanged(old, new client.Object) bool {
	oldPod, ok := old.(*corev1.Pod)
	if !ok {
		return false
	}
	newPod, ok := new.(*corev1.Pod)
	if !ok {
		return false
	}
	return pod.PodReady(oldPod) != pod.PodReady(newPod)
}
