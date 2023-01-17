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

	cron "github.com/robfig/cron/v3"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

func (r *Backup) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

//nolint
//+kubebuilder:webhook:path=/validate-mariadb-mmontes-io-v1alpha1-backup,mutating=false,failurePolicy=fail,sideEffects=None,groups=mariadb.mmontes.io,resources=backups,verbs=create;update,versions=v1alpha1,name=vbackup.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &Backup{}

var cronParser = cron.NewParser(
	cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow,
)

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *Backup) ValidateCreate() error {
	if err := r.validateSchedule(); err != nil {
		return err
	}
	if err := r.validateStorage(); err != nil {
		return err
	}
	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *Backup) ValidateUpdate(old runtime.Object) error {
	if err := inmutableWebhook.ValidateUpdate(r, old.(*Backup)); err != nil {
		return err
	}
	if err := r.validateSchedule(); err != nil {
		return err
	}
	if err := r.validateStorage(); err != nil {
		return err
	}
	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *Backup) ValidateDelete() error {
	return nil
}

func (r *Backup) validateSchedule() error {
	if r.Spec.Schedule == nil {
		return nil
	}
	if _, err := cronParser.Parse(r.Spec.Schedule.Cron); err != nil {
		return field.Invalid(
			field.NewPath("spec").Child("schedule").Child("cron"),
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
