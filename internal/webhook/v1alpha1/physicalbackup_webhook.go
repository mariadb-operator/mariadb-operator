package v1alpha1

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	k8sv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
)

// nolint:unused
// log is for logging in this package.
var physicalbackuplog = logf.Log.WithName("physicalbackup-resource")

// SetupPhysicalBackupWebhookWithManager registers the webhook for PhysicalBackup in the manager.
func SetupPhysicalBackupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).For(&k8sv1alpha1.PhysicalBackup{}).
		WithValidator(&PhysicalBackupCustomValidator{}).
		Complete()
}

// PhysicalBackupCustomValidator struct is responsible for validating the PhysicalBackup resource
// when it is created, updated, or deleted.
type PhysicalBackupCustomValidator struct{}

var _ webhook.CustomValidator = &PhysicalBackupCustomValidator{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type PhysicalBackup.
func (v *PhysicalBackupCustomValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	physicalbackup, ok := obj.(*k8sv1alpha1.PhysicalBackup)
	if !ok {
		return nil, fmt.Errorf("expected a PhysicalBackup object but got %T", obj)
	}
	physicalbackuplog.Info("Validation for PhysicalBackup upon creation", "name", physicalbackup.GetName())

	return nil, nil
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type PhysicalBackup.
func (v *PhysicalBackupCustomValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	physicalbackup, ok := newObj.(*k8sv1alpha1.PhysicalBackup)
	if !ok {
		return nil, fmt.Errorf("expected a PhysicalBackup object for the newObj but got %T", newObj)
	}
	physicalbackuplog.Info("Validation for PhysicalBackup upon update", "name", physicalbackup.GetName())

	return nil, nil
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type PhysicalBackup.
func (v *PhysicalBackupCustomValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	physicalbackup, ok := obj.(*k8sv1alpha1.PhysicalBackup)
	if !ok {
		return nil, fmt.Errorf("expected a PhysicalBackup object but got %T", obj)
	}
	physicalbackuplog.Info("Validation for PhysicalBackup upon deletion", "name", physicalbackup.GetName())

	return nil, nil
}
