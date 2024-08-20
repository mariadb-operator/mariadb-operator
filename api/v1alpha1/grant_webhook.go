package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func (r *Grant) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

//nolint
//+kubebuilder:webhook:path=/validate-k8s-mariadb-com-v1alpha1-grant,mutating=false,failurePolicy=fail,sideEffects=None,groups=k8s.mariadb.com,resources=grants,verbs=create;update,versions=v1alpha1,name=vgrant.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &Grant{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *Grant) ValidateCreate() (admission.Warnings, error) {
	if err := r.validateCleanupPolicy(); err != nil {
		return nil, err
	}
	return nil, nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *Grant) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	if err := inmutableWebhook.ValidateUpdate(r, old.(*Grant)); err != nil {
		return nil, err
	}
	if err := r.validateCleanupPolicy(); err != nil {
		return nil, err
	}
	return nil, nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *Grant) ValidateDelete() (admission.Warnings, error) {
	return nil, nil
}

func (r *Grant) validateCleanupPolicy() error {
	if r.Spec.CleanupPolicy != nil {
		if err := r.Spec.CleanupPolicy.Validate(); err != nil {
			return field.Invalid(
				field.NewPath("spec").Child("cleanupPolicy"),
				r.Spec.CleanupPolicy,
				err.Error(),
			)
		}
	}
	return nil
}
