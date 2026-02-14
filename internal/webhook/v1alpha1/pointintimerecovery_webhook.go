package v1alpha1

import (
	"context"
	"fmt"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// log is for logging in this package.
var pointintimerecoverylog = logf.Log.WithName("pointintimerecovery-resource")

// SetupPointInTimeRecoveryWebhookWithManager registers the webhook for PointInTimeRecovery in the manager.
func SetupPointInTimeRecoveryWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).For(&mariadbv1alpha1.PointInTimeRecovery{}).
		WithValidator(&PointInTimeRecoveryCustomValidator{}).
		Complete()
}

// +kubebuilder:webhook:path=/validate-k8s-mariadb-com-v1alpha1-pointintimerecovery,mutating=false,failurePolicy=fail,sideEffects=None,groups=k8s.mariadb.com,resources=pointintimerecoveries,verbs=create;update,versions=v1alpha1,name=vpointintimerecovery-v1alpha1.kb.io,admissionReviewVersions=v1

// PointInTimeRecoveryCustomValidator struct is responsible for validating the PointInTimeRecovery resource
// when it is created, updated, or deleted.
type PointInTimeRecoveryCustomValidator struct{}

var _ webhook.CustomValidator = &PointInTimeRecoveryCustomValidator{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type PointInTimeRecovery.
func (v *PointInTimeRecoveryCustomValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type PointInTimeRecovery.
func (v *PointInTimeRecoveryCustomValidator) ValidateUpdate(ctx context.Context, oldObj,
	newObj runtime.Object) (admission.Warnings, error) {
	pitr, ok := newObj.(*mariadbv1alpha1.PointInTimeRecovery)
	if !ok {
		return nil, fmt.Errorf("expected a PointInTimeRecovery object for the newObj but got %T", newObj)
	}
	oldPitr, ok := oldObj.(*mariadbv1alpha1.PointInTimeRecovery)
	if !ok {
		return nil, fmt.Errorf("expected a PointInTimeRecovery object for the oldObj but got %T", oldObj)
	}
	pointintimerecoverylog.V(1).Info("Validation for PointInTimeRecovery upon update", "name", pitr.GetName())

	return nil, inmutableWebhook.ValidateUpdate(pitr, oldPitr)
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type PointInTimeRecovery.
func (v *PointInTimeRecoveryCustomValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}
