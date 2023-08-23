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
	"github.com/mariadb-operator/mariadb-operator/pkg/builder"
	mariadbclient "github.com/mariadb-operator/mariadb-operator/pkg/client"
	"github.com/mariadb-operator/mariadb-operator/pkg/conditions"
	"github.com/mariadb-operator/mariadb-operator/pkg/controller/replication"
	"github.com/mariadb-operator/mariadb-operator/pkg/controller/secret"
	"github.com/mariadb-operator/mariadb-operator/pkg/health"
	"github.com/mariadb-operator/mariadb-operator/pkg/refresolver"
	"github.com/mariadb-operator/mariadb-operator/pkg/statefulset"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type Option func(*PodReplicationController)

func WithRefResolver(rr *refresolver.RefResolver) Option {
	return func(prc *PodReplicationController) {
		prc.refResolver = rr
	}
}

func WithSecretReconciler(sr *secret.SecretReconciler) Option {
	return func(prc *PodReplicationController) {
		prc.secretReconciler = sr
	}
}

func WithReplConfig(rc *replication.ReplicationConfig) Option {
	return func(prc *PodReplicationController) {
		prc.replConfig = rc
	}
}

// PodReplicationController reconciles a Pod object
type PodReplicationController struct {
	client.Client
	recorder         record.EventRecorder
	builder          *builder.Builder
	refResolver      *refresolver.RefResolver
	secretReconciler *secret.SecretReconciler
	replConfig       *replication.ReplicationConfig
}

func NewPodReplicationController(client client.Client, recorder record.EventRecorder, builder *builder.Builder,
	refResolver *refresolver.RefResolver, secretReconciler *secret.SecretReconciler,
	replConfig *replication.ReplicationConfig) PodReadinessController {
	return &PodReplicationController{
		Client:           client,
		recorder:         recorder,
		builder:          builder,
		refResolver:      refResolver,
		secretReconciler: secretReconciler,
		replConfig:       replConfig,
	}
}

func (r *PodReplicationController) ReconcilePodReady(ctx context.Context, pod corev1.Pod, mariadb *mariadbv1alpha1.MariaDB) error {
	if !r.shouldReconcile(mariadb) {
		return nil
	}
	if mariadb.Status.CurrentPrimaryPodIndex == nil {
		return errors.New("'status.currentPrimaryPodIndex' must be set")
	}
	log.FromContext(ctx).V(1).Info("Reconciling Pod in Ready state", "pod", pod.Name)

	index, err := statefulset.PodIndex(pod.Name)
	if err != nil {
		return fmt.Errorf("error getting Pod index: %v", err)
	}

	client, err := mariadbclient.NewInternalClientWithPodIndex(ctx, mariadb, r.refResolver, *index)
	if err != nil {
		return fmt.Errorf("error connecting to replica '%d': %v", *index, err)
	}
	defer client.Close()

	if *index == *mariadb.Status.CurrentPrimaryPodIndex {
		if err := r.replConfig.ConfigurePrimary(ctx, mariadb, client, *index); err != nil {
			return fmt.Errorf("error configuring primary in replica '%d': %v", *index, err)
		}
		return nil
	}
	if err := r.replConfig.ConfigureReplica(ctx, mariadb, client, *index, *mariadb.Status.CurrentPrimaryPodIndex); err != nil {
		return fmt.Errorf("error configuring replication in replica '%d': %v", *index, err)
	}
	return nil
}

func (r *PodReplicationController) ReconcilePodNotReady(ctx context.Context, pod corev1.Pod, mariadb *mariadbv1alpha1.MariaDB) error {
	if !r.shouldReconcile(mariadb) || !*mariadb.Replication().Primary.AutomaticFailover {
		return nil
	}
	logger := log.FromContext(ctx)
	logger.V(1).Info("Reconciling Pod in non Ready state", "pod", pod.Name)

	index, err := statefulset.PodIndex(pod.Name)
	if err != nil {
		return fmt.Errorf("error getting Pod index: %v", err)
	}
	if *index != *mariadb.Status.CurrentPrimaryPodIndex {
		return nil
	}

	fromIndex := mariadb.Status.CurrentPrimaryPodIndex
	toIndex, err := health.HealthyReplica(ctx, r, mariadb)
	if err != nil {
		return fmt.Errorf("error getting healthy replica: %v", err)
	}

	var errBundle *multierror.Error
	err = r.patch(ctx, mariadb, func(mdb *mariadbv1alpha1.MariaDB) {
		mdb.Replication().Primary.PodIndex = toIndex
	})
	errBundle = multierror.Append(errBundle, err)

	err = r.patchStatus(ctx, mariadb, func(status *mariadbv1alpha1.MariaDBStatus) {
		conditions.SetPrimarySwitching(status, mariadb)
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

func (r *PodReplicationController) shouldReconcile(mariadb *mariadbv1alpha1.MariaDB) bool {
	return mariadb.Replication().Enabled && mariadb.HasConfiguredReplication() && !mariadb.IsRestoringBackup()
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
