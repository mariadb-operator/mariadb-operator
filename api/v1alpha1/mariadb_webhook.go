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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

func (r *MariaDB) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

//nolint
//+kubebuilder:webhook:path=/validate-mariadb-mmontes-io-v1alpha1-mariadb,mutating=false,failurePolicy=fail,sideEffects=None,groups=mariadb.mmontes.io,resources=mariadbs,verbs=create;update,versions=v1alpha1,name=vmariadb.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &MariaDB{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *MariaDB) ValidateCreate() error {
	if err := r.validateReplication(); err != nil {
		return err
	}
	if err := r.validateBootstrapFrom(); err != nil {
		return err
	}
	if err := r.validatePodDisruptionBudget(); err != nil {
		return err
	}
	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *MariaDB) ValidateUpdate(old runtime.Object) error {
	oldMariadb := old.(*MariaDB)
	if err := r.validateReplication(); err != nil {
		return err
	}
	if err := r.validateBootstrapFrom(); err != nil {
		return err
	}
	if err := r.validatePodDisruptionBudget(); err != nil {
		return err
	}
	if err := r.validatePrimarySwitchover(oldMariadb); err != nil {
		return err
	}
	return inmutableWebhook.ValidateUpdate(r, oldMariadb)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *MariaDB) ValidateDelete() error {
	return nil
}

func (r *MariaDB) validateReplication() error {
	if r.Spec.Replication == nil && r.Spec.Replicas > 1 {
		return field.Invalid(
			field.NewPath("spec").Child("replicas"),
			r.Spec.Replicas,
			"Multiple replicas can only be specified when 'spec.replication' is configured",
		)
	}
	if r.Spec.Replication != nil {
		if r.Spec.Replicas <= 1 {
			return field.Invalid(
				field.NewPath("spec").Child("replicas"),
				r.Spec.Replicas,
				"Multiple replicas must be specified when 'spec.replication' is configured",
			)
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
