package v1alpha1

import (
	"slices"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func (r *User) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

//nolint
//+kubebuilder:webhook:path=/validate-k8s-mariadb-com-v1alpha1-user,mutating=false,failurePolicy=fail,sideEffects=None,groups=k8s.mariadb.com,resources=users,verbs=create;update,versions=v1alpha1,name=vuser.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &User{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *User) ValidateCreate() (admission.Warnings, error) {
	validateFns := []func() error{
		r.validatePassword,
		r.validateCleanupPolicy,
	}
	for _, fn := range validateFns {
		if err := fn(); err != nil {
			return nil, err
		}
	}

	return nil, nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *User) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	oldUser := old.(*User)

	if err := inmutableWebhook.ValidateUpdate(r, oldUser); err != nil {
		return nil, err
	}
	validateFns := []func() error{
		r.validatePassword,
		r.validateCleanupPolicy,
	}
	for _, fn := range validateFns {
		if err := fn(); err != nil {
			return nil, err
		}
	}

	return nil, nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *User) ValidateDelete() (admission.Warnings, error) {
	return nil, nil
}

func (r *User) validatePasswordPlugin() error {
	if r.Spec.PasswordPlugin.PluginArgSecretKeyRef != nil && r.Spec.PasswordPlugin.PluginNameSecretKeyRef == nil {
		return field.Invalid(
			field.NewPath("spec").Child("passwordPlugin").Child("pluginArgSecretKeyRef"),
			r.Spec.PasswordPlugin.PluginArgSecretKeyRef,
			"'spec.passwordPlugin.pluginArgSecretKeyRef' can only be set when 'spec.passwordPlugin.pluginNameSecretKeyRef' is set",
		)

	}
	return nil
}

func (r *User) validatePassword() error {
	if err := r.validatePasswordPlugin(); err != nil {
		return err
	}

	definedPasswordMethods := []bool{
		r.Spec.PasswordSecretKeyRef != nil,
		r.Spec.PasswordHashSecretKeyRef != nil,
		r.Spec.PasswordPlugin.PluginNameSecretKeyRef != nil,
	}
	definedPasswordMethods = slices.DeleteFunc(definedPasswordMethods, func(v bool) bool { return !v })

	if len(definedPasswordMethods) > 1 {
		return field.Invalid(
			field.NewPath("spec").Child("passwordSecretKeyRef"),
			r.Spec.PasswordSecretKeyRef,
			"Only one of 'spec.passwordSecretKeyRef', 'spec.passwordHashSecretKeyRef', or "+
				"'spec.passwordPlugin.pluginNameSecretKeyRef' can be defined",
		)
	}
	return nil
}

func (r *User) validateCleanupPolicy() error {
	if r.Spec.CleanupPolicy != nil {
		if err := r.Spec.CleanupPolicy.Validate(); err != nil {
			return field.Invalid(
				field.NewPath("spec").Child("cleanupPolicy"),
				r.Spec.CleanupPolicy,
				err.Error(),
			)
		}
	}
	return nil
}
