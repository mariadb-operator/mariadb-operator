package v1alpha1

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func (r *SqlJob) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

//nolint
//+kubebuilder:webhook:path=/validate-k8s-mariadb-com-v1alpha1-sqljob,mutating=false,failurePolicy=fail,sideEffects=None,groups=k8s.mariadb.com,resources=sqljobs,verbs=create;update,versions=v1alpha1,name=vsqljob.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &SqlJob{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (s *SqlJob) ValidateCreate() (admission.Warnings, error) {
	return s.validate()
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (s *SqlJob) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	if err := inmutableWebhook.ValidateUpdate(s, old.(*SqlJob)); err != nil {
		return nil, err
	}
	return s.validate()
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *SqlJob) ValidateDelete() (admission.Warnings, error) {
	return nil, nil
}

func (s *SqlJob) validate() (admission.Warnings, error) {
	if err := s.validateSql(); err != nil {
		return nil, err
	}
	if err := s.validateSchedule(); err != nil {
		return nil, err
	}
	return nil, nil
}

func (s *SqlJob) validateSql() error {
	if s.Spec.Sql == nil && s.Spec.SqlConfigMapKeyRef == nil {
		return field.Invalid(
			field.NewPath("spec"),
			s.Spec,
			"either `spec.sql` or `sql.sqlConfigMapKeyRef` must be set",
		)
	}
	return nil
}

func (s *SqlJob) validateSchedule() error {
	if s.Spec.Schedule == nil {
		return nil
	}
	if err := s.Spec.Schedule.Validate(); err != nil {
		return field.Invalid(
			field.NewPath("spec").Child("schedule"),
			s.Spec.Schedule,
			fmt.Sprintf("invalid schedule: %v", err),
		)
	}
	return nil
}
