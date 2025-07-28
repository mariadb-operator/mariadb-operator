package v1alpha1

import (
	"context"
	"fmt"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v25/api/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// log is for logging in this package.
var restorelog = logf.Log.WithName("restore-resource")

// SetupRestoreWebhookWithManager registers the webhook for Restore in the manager.
func SetupRestoreWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).For(&mariadbv1alpha1.Restore{}).
		WithValidator(&RestoreCustomValidator{}).
		Complete()
}

// +kubebuilder:webhook:path=/validate-k8s-mariadb-com-v1alpha1-restore,mutating=false,failurePolicy=fail,sideEffects=None,groups=k8s.mariadb.com,resources=restores,verbs=create;update,versions=v1alpha1,name=vrestore-v1alpha1.kb.io,admissionReviewVersions=v1

// RestoreCustomValidator struct is responsible for validating the Restore resource
// when it is created, updated, or deleted.
type RestoreCustomValidator struct{}

var _ webhook.CustomValidator = &RestoreCustomValidator{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type Restore.
func (v *RestoreCustomValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	restore, ok := obj.(*mariadbv1alpha1.Restore)
	if !ok {
		return nil, fmt.Errorf("expected a Restore object but got %T", obj)
	}
	restorelog.V(1).Info("Validation for Restore upon creation", "name", restore.GetName())

	return validateRestore(restore)
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type Restore.
func (v *RestoreCustomValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	restore, ok := newObj.(*mariadbv1alpha1.Restore)
	if !ok {
		return nil, fmt.Errorf("expected a Restore object for the newObj but got %T", newObj)
	}
	oldRestore, ok := oldObj.(*mariadbv1alpha1.Restore)
	if !ok {
		return nil, fmt.Errorf("expected a Restore object for the newObj but got %T", newObj)
	}
	restorelog.V(1).Info("Validation for Restore upon update", "name", restore.GetName())

	if err := inmutableWebhook.ValidateUpdate(restore, oldRestore); err != nil {
		return nil, err
	}
	return validateRestore(restore)
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type Restore.
func (v *RestoreCustomValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

func validateRestore(restore *mariadbv1alpha1.Restore) (admission.Warnings, error) {
	if err := restore.Spec.Validate(); err != nil {
		return nil, fmt.Errorf("invalid restore: %v", err)
	}
	return nil, nil
}
