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

package v1alpha1

import (
	"errors"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	log "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

var logger = log.Log.WithName("mariadb")

func (r *MariaDB) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

//nolint
//+kubebuilder:webhook:path=/mutate-mariadb-mmontes-io-v1alpha1-mariadb,mutating=true,failurePolicy=fail,sideEffects=None,groups=mariadb.mmontes.io,resources=mariadbs,verbs=create;update,versions=v1alpha1,name=mmariadb.kb.io,admissionReviewVersions=v1

var _ webhook.Defaulter = &MariaDB{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *MariaDB) Default() {
	logger.V(1).Info("Defaulting MariaDB", "mariadb", r.Name)
	if r.Spec.Galera == nil {
		return
	}
	if !r.Spec.Galera.Enabled {
		return
	}

	if r.Spec.Galera.Agent == nil {
		fiveSeconds := metav1.Duration{Duration: 5 * time.Second}
		r.Spec.Galera.Agent = &GaleraAgent{
			ContainerTemplate: ContainerTemplate{
				Image: Image{
					Repository: "ghcr.io/mariadb-operator/agent",
					Tag:        "v0.0.2",
					PullPolicy: corev1.PullIfNotPresent,
				},
			},
			Port: 5555,
			KubernetesAuth: &KubernetesAuth{
				Enabled:               true,
				AuthDelegatorRoleName: r.Name,
			},
			GracefulShutdownTimeout: &fiveSeconds,
		}
	}
	if r.Spec.Galera.Recovery == nil {
		oneMinute := metav1.Duration{Duration: 1 * time.Minute}
		fiveMinutes := metav1.Duration{Duration: 5 * time.Minute}
		threeMinutes := metav1.Duration{Duration: 3 * time.Minute}
		r.Spec.Galera.Recovery = &GaleraRecovery{
			Enabled:                 true,
			ClusterHealthyTimeout:   &oneMinute,
			ClusterBootstrapTimeout: &fiveMinutes,
			PodRecoveryTimeout:      &threeMinutes,
			PodSyncTimeout:          &threeMinutes,
		}
	}
	if r.Spec.Galera.InitContainer == nil {
		r.Spec.Galera.InitContainer = &ContainerTemplate{
			Image: Image{
				Repository: "ghcr.io/mariadb-operator/init",
				Tag:        "v0.0.2",
				PullPolicy: corev1.PullIfNotPresent,
			},
		}
	}
	if r.Spec.Galera.VolumeClaimTemplate == nil {
		defaultStorageClass := "default"
		r.Spec.Galera.VolumeClaimTemplate = &corev1.PersistentVolumeClaimSpec{
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					"storage": resource.MustParse("50Mi"),
				},
			},
			StorageClassName: &defaultStorageClass,
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
		}
	}
}

//nolint
//+kubebuilder:webhook:path=/validate-mariadb-mmontes-io-v1alpha1-mariadb,mutating=false,failurePolicy=fail,sideEffects=None,groups=mariadb.mmontes.io,resources=mariadbs,verbs=create;update,versions=v1alpha1,name=vmariadb.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &MariaDB{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *MariaDB) ValidateCreate() error {
	logger.V(1).Info("Validate MariaDB creation", "mariadb", r.Name)
	validateFns := []func() error{
		r.validateHA,
		r.validateGalera,
		r.validateReplication,
		r.validateBootstrapFrom,
		r.validatePodDisruptionBudget,
	}
	for _, fn := range validateFns {
		if err := fn(); err != nil {
			return err
		}
	}
	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *MariaDB) ValidateUpdate(old runtime.Object) error {
	logger.V(1).Info("Validate MariaDB update", "mariadb", r.Name)
	validateFns := []func() error{
		r.validateHA,
		r.validateGalera,
		r.validateReplication,
		r.validateBootstrapFrom,
		r.validatePodDisruptionBudget,
	}
	for _, fn := range validateFns {
		if err := fn(); err != nil {
			return err
		}
	}
	oldMariadb := old.(*MariaDB)
	if err := r.validatePrimarySwitchover(oldMariadb); err != nil {
		return err
	}
	return inmutableWebhook.ValidateUpdate(r, oldMariadb)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *MariaDB) ValidateDelete() error {
	return nil
}

func (r *MariaDB) validateHA() error {
	if r.Spec.Replication != nil && r.IsGaleraEnabled() {
		return errors.New("You may only enable one HA method at a time, either 'spec.replication' or 'spec.galera'")
	}
	if !r.IsHAEnabled() && r.Spec.Replicas > 1 {
		return field.Invalid(
			field.NewPath("spec").Child("replicas"),
			r.Spec.Replicas,
			"Multiple replicas can only be specified when 'spec.replication' or 'spec.galera' are configured",
		)
	}
	if r.IsHAEnabled() && r.Spec.Replicas <= 1 {
		return field.Invalid(
			field.NewPath("spec").Child("replicas"),
			r.Spec.Replicas,
			"Multiple replicas must be specified when 'spec.replication' or 'spec.galera' are configured",
		)
	}
	return nil
}

func (r *MariaDB) validateGalera() error {
	if r.Spec.Galera == nil {
		return nil
	}
	if err := r.Spec.Galera.SST.Validate(); err != nil {
		return field.Invalid(
			field.NewPath("spec").Child("galera").Child("sst"),
			r.Spec.Galera.SST,
			err.Error(),
		)
	}
	if r.Spec.Galera.ReplicaThreads < 1 {
		return field.Invalid(
			field.NewPath("spec").Child("galera").Child("replicaThreads"),
			r.Spec.Galera.ReplicaThreads,
			"'spec.galera.replicaThreads' must be at least 1",
		)
	}
	return nil
}

func (r *MariaDB) validateReplication() error {
	if r.Spec.Replication == nil {
		return nil
	}
	if r.Spec.Replication.Primary.PodIndex < 0 || r.Spec.Replication.Primary.PodIndex >= int(r.Spec.Replicas) {
		return field.Invalid(
			field.NewPath("spec").Child("replication").Child("primary").Child("podIndex"),
			r.Spec.Replication.Primary.PodIndex,
			"'spec.replication.primary.podIndex' out of 'spec.replicas' bounds",
		)
	}
	if err := r.Spec.Replication.Replica.Validate(); err != nil {
		return field.Invalid(
			field.NewPath("spec").Child("replication").Child("replica"),
			r.Spec.Replication,
			err.Error(),
		)
	}
	return nil
}

func (r *MariaDB) validatePrimarySwitchover(oldMariadb *MariaDB) error {
	if oldMariadb.Spec.Replication != nil && oldMariadb.IsSwitchingPrimary() {
		if oldMariadb.Spec.Replication.Primary.PodIndex != r.Spec.Replication.Primary.PodIndex {
			return field.Invalid(
				field.NewPath("spec").Child("replication").Child("primary").Child("podIndex"),
				r.Spec.Replication.Primary.PodIndex,
				"'spec.replication.primary.podIndex' cannot be updated during a primary switchover",
			)
		}
		if oldMariadb.Spec.Replication.Primary.AutomaticFailover != r.Spec.Replication.Primary.AutomaticFailover &&
			r.Spec.Replication.Primary.AutomaticFailover {
			return field.Invalid(
				field.NewPath("spec").Child("replication").Child("primary").Child("automaticFailover"),
				r.Spec.Replication.Primary.PodIndex,
				"'spec.replication.primary.automaticFailover' cannot be enabled during a primary switchover",
			)
		}
	}
	return nil
}

func (r *MariaDB) validateBootstrapFrom() error {
	if r.Spec.BootstrapFrom == nil {
		return nil
	}
	if err := r.Spec.BootstrapFrom.Validate(); err != nil {
		return field.Invalid(
			field.NewPath("spec").Child("bootstrapFrom"),
			r.Spec.BootstrapFrom,
			err.Error(),
		)
	}
	return nil
}

func (r *MariaDB) validatePodDisruptionBudget() error {
	if r.Spec.PodDisruptionBudget == nil {
		return nil
	}
	if err := r.Spec.PodDisruptionBudget.Validate(); err != nil {
		return field.Invalid(
			field.NewPath("spec").Child("podDisruptionBudget"),
			r.Spec.PodDisruptionBudget,
			err.Error(),
		)
	}
	return nil
}
