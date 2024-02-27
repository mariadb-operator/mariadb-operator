package v1alpha1

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func (r *Restore) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

//nolint
//+kubebuilder:webhook:path=/validate-k8s-mariadb-com-v1alpha1-restore,mutating=false,failurePolicy=fail,sideEffects=None,groups=k8s.mariadb.com,resources=restores,verbs=create;update,versions=v1alpha1,name=vrestore.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &Restore{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *Restore) ValidateCreate() (admission.Warnings, error) {
	return r.validate()
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *Restore) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	if err := inmutableWebhook.ValidateUpdate(r, old.(*Restore)); err != nil {
		return nil, err
	}
	return r.validate()
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *Restore) ValidateDelete() (admission.Warnings, error) {
	return nil, nil
}

func (r *Restore) validate() (admission.Warnings, error) {
	if err := r.Spec.RestoreSource.Validate(); err != nil {
		return nil, fmt.Errorf("invalid restore: %v", err)
	}
	return nil, nil
}
