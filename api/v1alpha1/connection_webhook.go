package v1alpha1

import (
	"fmt"
	"text/template"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func (r *Connection) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

//nolint
//+kubebuilder:webhook:path=/validate-k8s-mariadb-com-v1alpha1-connection,mutating=false,failurePolicy=fail,sideEffects=None,groups=k8s.mariadb.com,resources=connections,verbs=create;update,versions=v1alpha1,name=vconnection.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &Connection{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *Connection) ValidateCreate() (admission.Warnings, error) {
	return r.validate()
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *Connection) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	if err := inmutableWebhook.ValidateUpdate(r, old.(*Connection)); err != nil {
		return nil, err
	}
	return r.validate()
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *Connection) ValidateDelete() (admission.Warnings, error) {
	return nil, nil
}

func (r *Connection) validate() (admission.Warnings, error) {
	if err := r.validateRefs(); err != nil {
		return nil, err
	}
	if err := r.validateHealthCheck(); err != nil {
		return nil, err
	}
	return nil, r.validateCustomDSNFormat()
}

func (r *Connection) validateRefs() error {
	if r.Spec.MariaDBRef == nil && r.Spec.MaxScaleRef == nil {
		return field.Invalid(
			field.NewPath("spec").Child("mariaDbRef"),
			r.Spec.MariaDBRef,
			"'spec.mariaDbRef' or 'spec.maxScaleRef' must be defined",
		)
	}
	if r.Spec.MariaDBRef != nil && r.Spec.MaxScaleRef != nil {
		return field.Invalid(
			field.NewPath("spec").Child("mariaDbRef"),
			r.Spec.MariaDBRef,
			"'spec.mariaDbRef' and 'spec.maxScaleRef' cannot be specified simultaneously",
		)
	}
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
