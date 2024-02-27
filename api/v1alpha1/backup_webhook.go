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
//+kubebuilder:webhook:path=/validate-k8s-mariadb-com-v1alpha1-backup,mutating=false,failurePolicy=fail,sideEffects=None,groups=k8s.mariadb.com,resources=backups,verbs=create;update,versions=v1alpha1,name=vbackup.kb.io,admissionReviewVersions=v1

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
	if err := r.Validate(); err != nil {
		return nil, field.Invalid(
			field.NewPath("spec"),
			r.Spec,
			fmt.Sprintf("invalid Backup: %v", err),
		)
	}
	return nil, nil
}
