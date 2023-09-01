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
	"text/template"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

func (r *Connection) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

//nolint
//+kubebuilder:webhook:path=/validate-mariadb-mmontes-io-v1alpha1-connection,mutating=false,failurePolicy=fail,sideEffects=None,groups=mariadb.mmontes.io,resources=connections,verbs=create;update,versions=v1alpha1,name=vconnection.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &Connection{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *Connection) ValidateCreate() error {
	if err := r.validateHealthCheck(); err != nil {
		return err
	}
	if err := r.validateCustomDSNFormat(); err != nil {
		return err
	}
	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *Connection) ValidateUpdate(old runtime.Object) error {
	if err := r.validateHealthCheck(); err != nil {
		return err
	}
	if err := r.validateCustomDSNFormat(); err != nil {
		return err
	}
	return inmutableWebhook.ValidateUpdate(r, old.(*Connection))
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *Connection) ValidateDelete() error {
	return nil
}

func (r *Connection) validateHealthCheck() error {
	if r.Spec.HealthCheck == nil {
		return nil
	}
	if r.Spec.HealthCheck.Interval != nil {
		duration := r.Spec.HealthCheck.Interval.Duration.String()
		if _, err := time.ParseDuration(duration); err != nil {
			return field.Invalid(
				field.NewPath("spec").Child("healthCheck").Child("interval"),
				r.Spec.HealthCheck.Interval,
				fmt.Sprintf("invalid duration: '%s'", duration),
			)
		}
	}
	if r.Spec.HealthCheck.RetryInterval != nil {
		duration := r.Spec.HealthCheck.RetryInterval.Duration.String()
		if _, err := time.ParseDuration(duration); err != nil {
			return field.Invalid(
				field.NewPath("spec").Child("healthCheck").Child("retryInterval"),
				r.Spec.HealthCheck.RetryInterval,
				fmt.Sprintf("invalid duration: '%s'", duration),
			)
		}
	}
	return nil
}

func (r *Connection) validateCustomDSNFormat() error {
	if r.Spec.SecretTemplate == nil || r.Spec.SecretTemplate.Format == nil {
		return nil
	}

	_, err := template.New("").Parse(*r.Spec.SecretTemplate.Format)
	if err != nil {
		return field.Invalid(
			field.NewPath("spec").Child("secretTemplate").Child("format"),
			r.Spec.SecretTemplate.Format,
			fmt.Sprintf("invalid format template: '%s'", err),
		)
	}

	return nil
}
