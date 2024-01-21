package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var maxscaleLogger = logf.Log.WithName("maxscale")

func (r *MaxScale) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

//+kubebuilder:webhook:path=/validate-mariadb-mmontes-io-v1alpha1-maxscale,mutating=false,failurePolicy=fail,sideEffects=None,groups=mariadb.mmontes.io,resources=maxscales,verbs=create;update,versions=v1alpha1,name=vmaxscale.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &MaxScale{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *MaxScale) ValidateCreate() (admission.Warnings, error) {
	maxscaleLogger.V(1).Info("Validate create", "name", r.Name)
	return nil, nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *MaxScale) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	maxscaleLogger.V(1).Info("Validate update", "name", r.Name)
	oldMaxScale := old.(*MaxScale)
	if err := inmutableWebhook.ValidateUpdate(r, oldMaxScale); err != nil {
		return nil, err
	}
	return nil, nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *MaxScale) ValidateDelete() (admission.Warnings, error) {
	return nil, nil
}
