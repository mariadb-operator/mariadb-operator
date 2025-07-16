package v1alpha1

import (
	"context"
	"fmt"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// log is for logging in this package.
var sqljoblog = logf.Log.WithName("sqljob-resource")

// SetupSQLJobWebhookWithManager registers the webhook for SqlJob in the manager.
func SetupSQLJobWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).For(&mariadbv1alpha1.SQLJob{}).
		WithValidator(&SQLJobCustomValidator{}).
		Complete()
}

// +kubebuilder:webhook:path=/validate-k8s-mariadb-com-v1alpha1-sqljob,mutating=false,failurePolicy=fail,sideEffects=None,groups=k8s.mariadb.com,resources=sqljobs,verbs=create;update,versions=v1alpha1,name=vsqljob-v1alpha1.kb.io,admissionReviewVersions=v1

// SqlJobCustomValidator struct is responsible for validating the SqlJob resource
// when it is created, updated, or deleted.
type SQLJobCustomValidator struct{}

var _ webhook.CustomValidator = &SQLJobCustomValidator{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type SqlJob.
func (v *SQLJobCustomValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	sqljob, ok := obj.(*mariadbv1alpha1.SQLJob)
	if !ok {
		return nil, fmt.Errorf("expected a SqlJob object but got %T", obj)
	}
	sqljoblog.V(1).Info("Validation for SqlJob upon creation", "name", sqljob.GetName())

	return validateSQLJob(sqljob)
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type SqlJob.
func (v *SQLJobCustomValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	sqljob, ok := newObj.(*mariadbv1alpha1.SQLJob)
	if !ok {
		return nil, fmt.Errorf("expected a SqlJob object for the newObj but got %T", newObj)
	}
	oldSqljob, ok := oldObj.(*mariadbv1alpha1.SQLJob)
	if !ok {
		return nil, fmt.Errorf("expected a SqlJob object for the newObj but got %T", newObj)
	}
	sqljoblog.V(1).Info("Validation for SqlJob upon update", "name", sqljob.GetName())

	if err := inmutableWebhook.ValidateUpdate(sqljob, oldSqljob); err != nil {
		return nil, err
	}
	return validateSQLJob(sqljob)
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type SqlJob.
func (v *SQLJobCustomValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

func validateSQLJob(sqlJob *mariadbv1alpha1.SQLJob) (admission.Warnings, error) {
	if err := validateSQL(sqlJob); err != nil {
		return nil, err
	}
	if err := validateSQLJobSchedule(sqlJob); err != nil {
		return nil, err
	}
	return nil, nil
}

func validateSQL(sqlJob *mariadbv1alpha1.SQLJob) error {
	if sqlJob.Spec.SQL == nil && sqlJob.Spec.SQLConfigMapKeyRef == nil {
		return field.Invalid(
			field.NewPath("spec"),
			sqlJob.Spec,
			"either `spec.sql` or `sql.sqlConfigMapKeyRef` must be set",
		)
	}
	return nil
}

func validateSQLJobSchedule(sqlJob *mariadbv1alpha1.SQLJob) error {
	if sqlJob.Spec.Schedule == nil {
		return nil
	}
	if err := sqlJob.Spec.Schedule.Validate(); err != nil {
		return field.Invalid(
			field.NewPath("spec").Child("schedule"),
			sqlJob.Spec.Schedule,
			fmt.Sprintf("invalid schedule: %v", err),
		)
	}
	return nil
}
