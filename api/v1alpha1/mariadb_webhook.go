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
	if r.Spec.Replication != nil && r.Spec.Replication.Enabled {
		logger.V(1).Info("Defaulting spec.replication", "mariadb", r.Name)
		r.Spec.Replication.FillWithDefaults()
		return
	}
	if r.Spec.Galera != nil && r.Spec.Galera.Enabled {
		logger.V(1).Info("Defaulting spec.galera", "mariadb", r.Name)
		r.Spec.Galera.FillWithDefaults()
		return
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
	if r.Replication().Enabled && r.Galera().Enabled {
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
	if !r.Galera().Enabled {
		return nil
	}
	if *r.Galera().Primary.PodIndex < 0 || *r.Galera().Primary.PodIndex >= int(r.Spec.Replicas) {
		return field.Invalid(
			field.NewPath("spec").Child("galera").Child("primary").Child("podIndex"),
			r.Replication().Primary.PodIndex,
			"'spec.galera.primary.podIndex' out of 'spec.replicas' bounds",
		)
	}
	if err := r.Galera().SST.Validate(); err != nil {
		return field.Invalid(
			field.NewPath("spec").Child("galera").Child("sst"),
			r.Galera().SST,
			err.Error(),
		)
	}
	if *r.Galera().ReplicaThreads < 1 {
		return field.Invalid(
			field.NewPath("spec").Child("galera").Child("replicaThreads"),
			r.Galera().ReplicaThreads,
			"'spec.galera.replicaThreads' must be at least 1",
		)
	}
	return nil
}

func (r *MariaDB) validateReplication() error {
	if !r.Replication().Enabled {
		return nil
	}
	if *r.Replication().Primary.PodIndex < 0 || *r.Replication().Primary.PodIndex >= int(r.Spec.Replicas) {
		return field.Invalid(
			field.NewPath("spec").Child("replication").Child("primary").Child("podIndex"),
			r.Replication().Primary.PodIndex,
			"'spec.replication.primary.podIndex' out of 'spec.replicas' bounds",
		)
	}
	if err := r.Replication().Replica.Validate(); err != nil {
		return field.Invalid(
			field.NewPath("spec").Child("replication").Child("replica"),
			r.Replication(),
			err.Error(),
		)
	}
	return nil
}

func (r *MariaDB) validatePrimarySwitchover(old *MariaDB) error {
	if old.Replication().Enabled && old.IsSwitchingPrimary() {
		if *old.Replication().Primary.PodIndex != *r.Replication().Primary.PodIndex {
			return field.Invalid(
				field.NewPath("spec").Child("replication").Child("primary").Child("podIndex"),
				r.Replication().Primary.PodIndex,
				"'spec.replication.primary.podIndex' cannot be updated during a primary switchover",
			)
		}
		if *old.Replication().Primary.AutomaticFailover != *r.Replication().Primary.AutomaticFailover &&
			*r.Replication().Primary.AutomaticFailover {
			return field.Invalid(
				field.NewPath("spec").Child("replication").Child("primary").Child("automaticFailover"),
				r.Replication().Primary.PodIndex,
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
