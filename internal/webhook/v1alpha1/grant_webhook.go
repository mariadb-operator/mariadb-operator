package v1alpha1

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
)

// log is for logging in this package.
var grantlog = logf.Log.WithName("grant-resource")

// SetupGrantWebhookWithManager registers the webhook for Grant in the manager.
func SetupGrantWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).For(&v1alpha1.Grant{}).
		WithValidator(&GrantCustomValidator{}).
		Complete()
}

// +kubebuilder:webhook:path=/validate-k8s-mariadb-com-v1alpha1-grant,mutating=false,failurePolicy=fail,sideEffects=None,groups=k8s.mariadb.com,resources=grants,verbs=create;update,versions=v1alpha1,name=vgrant-v1alpha1.kb.io,admissionReviewVersions=v1

// GrantCustomValidator struct is responsible for validating the Grant resource
// when it is created, updated, or deleted.
type GrantCustomValidator struct{}

var _ webhook.CustomValidator = &GrantCustomValidator{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type Grant.
func (v *GrantCustomValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	grant, ok := obj.(*v1alpha1.Grant)
	if !ok {
		return nil, fmt.Errorf("expected a Grant object but got %T", obj)
	}
	grantlog.Info("Validation for Grant upon creation", "name", grant.GetName())

	if err := validateGrantCleanupPolicy(grant); err != nil {
		return nil, err
	}
	return nil, nil
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type Grant.
func (v *GrantCustomValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	grant, ok := newObj.(*v1alpha1.Grant)
	if !ok {
		return nil, fmt.Errorf("expected a Grant object for the newObj but got %T", newObj)
	}
	oldGrant, ok := oldObj.(*v1alpha1.Grant)
	if !ok {
		return nil, fmt.Errorf("expected a Grant object for the newObj but got %T", newObj)
	}
	grantlog.Info("Validation for Grant upon update", "name", grant.GetName())

	if err := inmutableWebhook.ValidateUpdate(grant, oldGrant); err != nil {
		return nil, err
	}
	if err := validateGrantCleanupPolicy(grant); err != nil {
		return nil, err
	}
	return nil, nil
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type Grant.
func (v *GrantCustomValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	grant, ok := obj.(*v1alpha1.Grant)
	if !ok {
		return nil, fmt.Errorf("expected a Grant object but got %T", obj)
	}
	grantlog.Info("Validation for Grant upon deletion", "name", grant.GetName())

	return nil, nil
}

func validateGrantCleanupPolicy(grant *v1alpha1.Grant) error {
	if grant.Spec.CleanupPolicy != nil {
		if err := grant.Spec.CleanupPolicy.Validate(); err != nil {
			return field.Invalid(
				field.NewPath("spec").Child("cleanupPolicy"),
				grant.Spec.CleanupPolicy,
				err.Error(),
			)
		}
	}
	return nil
}
