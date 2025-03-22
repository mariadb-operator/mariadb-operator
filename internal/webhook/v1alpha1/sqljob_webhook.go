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
	k8sv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
)

// log is for logging in this package.
var sqljoblog = logf.Log.WithName("sqljob-resource")

// SetupSqlJobWebhookWithManager registers the webhook for SqlJob in the manager.
func SetupSqlJobWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).For(&k8sv1alpha1.SqlJob{}).
		WithValidator(&SqlJobCustomValidator{}).
		Complete()
}

// +kubebuilder:webhook:path=/validate-k8s-mariadb-com-v1alpha1-sqljob,mutating=false,failurePolicy=fail,sideEffects=None,groups=k8s.mariadb.com,resources=sqljobs,verbs=create;update,versions=v1alpha1,name=vsqljob-v1alpha1.kb.io,admissionReviewVersions=v1

// SqlJobCustomValidator struct is responsible for validating the SqlJob resource
// when it is created, updated, or deleted.
type SqlJobCustomValidator struct{}

var _ webhook.CustomValidator = &SqlJobCustomValidator{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type SqlJob.
func (v *SqlJobCustomValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	sqljob, ok := obj.(*k8sv1alpha1.SqlJob)
	if !ok {
		return nil, fmt.Errorf("expected a SqlJob object but got %T", obj)
	}
	sqljoblog.Info("Validation for SqlJob upon creation", "name", sqljob.GetName())

	return validateSqlJob(sqljob)
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type SqlJob.
func (v *SqlJobCustomValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	sqljob, ok := newObj.(*k8sv1alpha1.SqlJob)
	if !ok {
		return nil, fmt.Errorf("expected a SqlJob object for the newObj but got %T", newObj)
	}
	oldSqljob, ok := oldObj.(*k8sv1alpha1.SqlJob)
	if !ok {
		return nil, fmt.Errorf("expected a SqlJob object for the newObj but got %T", newObj)
	}
	sqljoblog.Info("Validation for SqlJob upon update", "name", sqljob.GetName())

	if err := inmutableWebhook.ValidateUpdate(sqljob, oldSqljob); err != nil {
		return nil, err
	}
	return validateSqlJob(sqljob)
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type SqlJob.
func (v *SqlJobCustomValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	sqljob, ok := obj.(*k8sv1alpha1.SqlJob)
	if !ok {
		return nil, fmt.Errorf("expected a SqlJob object but got %T", obj)
	}
	sqljoblog.Info("Validation for SqlJob upon deletion", "name", sqljob.GetName())

	return nil, nil
}

func validateSqlJob(sqlJob *v1alpha1.SqlJob) (admission.Warnings, error) {
	if err := validateSql(sqlJob); err != nil {
		return nil, err
	}
	if err := validateSqlJobSchedule(sqlJob); err != nil {
		return nil, err
	}
	return nil, nil
}

func validateSql(sqlJob *v1alpha1.SqlJob) error {
	if sqlJob.Spec.Sql == nil && sqlJob.Spec.SqlConfigMapKeyRef == nil {
		return field.Invalid(
			field.NewPath("spec"),
			sqlJob.Spec,
			"either `spec.sql` or `sql.sqlConfigMapKeyRef` must be set",
		)
	}
	return nil
}

func validateSqlJobSchedule(sqlJob *v1alpha1.SqlJob) error {
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
