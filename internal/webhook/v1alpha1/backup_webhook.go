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
var backuplog = logf.Log.WithName("backup-resource")

// SetupBackupWebhookWithManager registers the webhook for Backup in the manager.
func SetupBackupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).For(&v1alpha1.Backup{}).
		WithValidator(&BackupCustomValidator{}).
		Complete()
}

// +kubebuilder:webhook:path=/validate-k8s-mariadb-com-v1alpha1-backup,mutating=false,failurePolicy=fail,sideEffects=None,groups=k8s.mariadb.com,resources=backups,verbs=create;update,versions=v1alpha1,name=vbackup-v1alpha1.kb.io,admissionReviewVersions=v1

// BackupCustomValidator struct is responsible for validating the Backup resource
// when it is created, updated, or deleted.
type BackupCustomValidator struct{}

var _ webhook.CustomValidator = &BackupCustomValidator{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type Backup.
func (v *BackupCustomValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	backup, ok := obj.(*v1alpha1.Backup)
	if !ok {
		return nil, fmt.Errorf("expected a Backup object but got %T", obj)
	}
	backuplog.Info("Validation for Backup upon creation", "name", backup.GetName())

	return validateBackup(backup)
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type Backup.
func (v *BackupCustomValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	backup, ok := newObj.(*v1alpha1.Backup)
	if !ok {
		return nil, fmt.Errorf("expected a Backup object for the newObj but got %T", newObj)
	}
	oldBackup, ok := oldObj.(*v1alpha1.Backup)
	if !ok {
		return nil, fmt.Errorf("expected a Backup object for the newObj but got %T", newObj)
	}
	backuplog.Info("Validation for Backup upon update", "name", backup.GetName())

	if err := inmutableWebhook.ValidateUpdate(backup, oldBackup); err != nil {
		return nil, err
	}
	return validateBackup(backup)
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type Backup.
func (v *BackupCustomValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	backup, ok := obj.(*v1alpha1.Backup)
	if !ok {
		return nil, fmt.Errorf("expected a Backup object but got %T", obj)
	}
	backuplog.Info("Validation for Backup upon deletion", "name", backup.GetName())

	return nil, nil
}

func validateBackup(backup *v1alpha1.Backup) (admission.Warnings, error) {
	if err := backup.Validate(); err != nil {
		return nil, field.Invalid(
			field.NewPath("spec"),
			backup.Spec,
			fmt.Sprintf("invalid Backup: %v", err),
		)
	}
	return nil, nil
}
