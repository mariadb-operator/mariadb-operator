package v1alpha1

import (
	"context"
	"fmt"

	"github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// log is for logging in this package.
var physicalbackuplog = logf.Log.WithName("physicalbackup-resource")

// SetupPhysicalBackupWebhookWithManager registers the webhook for PhysicalBackup in the manager.
func SetupPhysicalBackupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).For(&mariadbv1alpha1.PhysicalBackup{}).
		WithValidator(&PhysicalBackupCustomValidator{}).
		Complete()
}

// +kubebuilder:webhook:path=/validate-k8s-mariadb-com-v1alpha1-physicalbackup,mutating=false,failurePolicy=fail,sideEffects=None,groups=k8s.mariadb.com,resources=physicalbackups,verbs=create;update,versions=v1alpha1,name=vphysicalbackup-v1alpha1.kb.io,admissionReviewVersions=v1

// PhysicalBackupCustomValidator struct is responsible for validating the PhysicalBackup resource
// when it is created, updated, or deleted.
type PhysicalBackupCustomValidator struct{}

var _ webhook.CustomValidator = &PhysicalBackupCustomValidator{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type PhysicalBackup.
func (v *PhysicalBackupCustomValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	physicalBackup, ok := obj.(*mariadbv1alpha1.PhysicalBackup)
	if !ok {
		return nil, fmt.Errorf("expected a PhysicalBackup object but got %T", obj)
	}
	physicalbackuplog.V(1).Info("Validation for PhysicalBackup upon creation", "name", physicalBackup.GetName())

	return validatePhysicalBackup(physicalBackup)
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type PhysicalBackup.
func (v *PhysicalBackupCustomValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	physicalBackup, ok := newObj.(*mariadbv1alpha1.PhysicalBackup)
	if !ok {
		return nil, fmt.Errorf("expected a PhysicalBackup object for the newObj but got %T", newObj)
	}
	oldPhysicalBackup, ok := oldObj.(*v1alpha1.PhysicalBackup)
	if !ok {
		return nil, fmt.Errorf("expected a PhysicalBackup object for the newObj but got %T", newObj)
	}
	physicalbackuplog.V(1).Info("Validation for PhysicalBackup upon update", "name", physicalBackup.GetName())

	if err := inmutableWebhook.ValidateUpdate(physicalBackup, oldPhysicalBackup); err != nil {
		return nil, err
	}
	return validatePhysicalBackup(physicalBackup)
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type PhysicalBackup.
func (v *PhysicalBackupCustomValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

func validatePhysicalBackup(backup *mariadbv1alpha1.PhysicalBackup) (admission.Warnings, error) {
	if err := backup.Validate(); err != nil {
		return nil, field.Invalid(
			field.NewPath("spec"),
			backup.Spec,
			fmt.Sprintf("invalid PhysicalBackup: %v", err),
		)
	}
	return nil, nil
}
