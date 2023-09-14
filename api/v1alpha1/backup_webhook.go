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
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func (r *Backup) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

//nolint
//+kubebuilder:webhook:path=/validate-mariadb-mmontes-io-v1alpha1-backup,mutating=false,failurePolicy=fail,sideEffects=None,groups=mariadb.mmontes.io,resources=backups,verbs=create;update,versions=v1alpha1,name=vbackup.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &Backup{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *Backup) ValidateCreate() (admission.Warnings, error) {
	return r.validate()
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *Backup) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	if err := inmutableWebhook.ValidateUpdate(r, old.(*Backup)); err != nil {
		return nil, err
	}
	return r.validate()
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *Backup) ValidateDelete() (admission.Warnings, error) {
	return nil, nil
}

func (r *Backup) validate() (admission.Warnings, error) {
	if err := r.validateSchedule(); err != nil {
		return nil, err
	}
	return nil, r.validateStorage()
}

func (r *Backup) validateSchedule() error {
	if r.Spec.Schedule == nil {
		return nil
	}
	if err := r.Spec.Schedule.Validate(); err != nil {
		return field.Invalid(
			field.NewPath("spec").Child("schedule"),
			r.Spec.Schedule,
			fmt.Sprintf("invalid schedule: %v", err),
		)
	}
	return nil
}

func (r *Backup) validateStorage() error {
	if err := r.Spec.Storage.Validate(); err != nil {
		return field.Invalid(
			field.NewPath("spec").Child("storage"),
			r.Spec.Storage,
			fmt.Sprintf("invalid storage: %v", err),
		)
	}
	return nil
}
