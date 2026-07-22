package v1alpha1

import (
	"context"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// log is for logging in this package.
var databaselog = logf.Log.WithName("database-resource")

// SetupDatabaseWebhookWithManager registers the webhook for Database in the manager.
func SetupDatabaseWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &mariadbv1alpha1.Database{}).
		WithValidator(&DatabaseCustomValidator{}).
		Complete()
}

// +kubebuilder:webhook:path=/validate-k8s-mariadb-com-v1alpha1-database,mutating=false,failurePolicy=fail,sideEffects=None,groups=k8s.mariadb.com,resources=databases,verbs=create;update,versions=v1alpha1,name=vdatabase-v1alpha1.kb.io,admissionReviewVersions=v1

// DatabaseCustomValidator struct is responsible for validating the Database resource
// when it is created, updated, or deleted.
type DatabaseCustomValidator struct{}

var _ admission.Validator[*mariadbv1alpha1.Database] = &DatabaseCustomValidator{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type Database.
func (v *DatabaseCustomValidator) ValidateCreate(ctx context.Context, database *mariadbv1alpha1.Database) (admission.Warnings, error) {
	databaselog.V(1).Info("Validation for Database upon creation", "name", database.GetName())

	if err := validateDatabaseCleanupPolicy(database); err != nil {
		return nil, err
	}
	return nil, nil
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type Database.
func (v *DatabaseCustomValidator) ValidateUpdate(ctx context.Context,
	oldDatabase, database *mariadbv1alpha1.Database) (admission.Warnings, error) {
	databaselog.V(1).Info("Validation for Database upon update", "name", database.GetName())

	if err := immutableWebhook.ValidateUpdate(database, oldDatabase); err != nil {
		return nil, err
	}
	if err := validateDatabaseCleanupPolicy(database); err != nil {
		return nil, err
	}
	return nil, nil
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type Database.
func (v *DatabaseCustomValidator) ValidateDelete(ctx context.Context, database *mariadbv1alpha1.Database) (admission.Warnings, error) {
	return nil, nil
}

func validateDatabaseCleanupPolicy(database *mariadbv1alpha1.Database) error {
	if database.Spec.CleanupPolicy != nil {
		if err := database.Spec.CleanupPolicy.Validate(); err != nil {
			return field.Invalid(
				field.NewPath("spec").Child("cleanupPolicy"),
				database.Spec.CleanupPolicy,
				err.Error(),
			)
		}
	}
	return nil
}
