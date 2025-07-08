package v1alpha1

import (
	"context"
	"fmt"
	"slices"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// log is for logging in this package.
var userlog = logf.Log.WithName("user-resource")

// SetupUserWebhookWithManager registers the webhook for User in the manager.
func SetupUserWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).For(&mariadbv1alpha1.User{}).
		WithValidator(&UserCustomValidator{}).
		Complete()
}

// +kubebuilder:webhook:path=/validate-k8s-mariadb-com-mariadbv1alpha1-user,mutating=false,failurePolicy=fail,sideEffects=None,groups=k8s.mariadb.com,resources=users,verbs=create;update,versions=mariadbv1alpha1,name=vuser-mariadbv1alpha1.kb.io,admissionReviewVersions=v1

// UserCustomValidator struct is responsible for validating the User resource
// when it is created, updated, or deleted.
type UserCustomValidator struct{}

var _ webhook.CustomValidator = &UserCustomValidator{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type User.
func (v *UserCustomValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	user, ok := obj.(*mariadbv1alpha1.User)
	if !ok {
		return nil, fmt.Errorf("expected a User object but got %T", obj)
	}
	userlog.V(1).Info("Validation for User upon creation", "name", user.GetName())

	validateFns := []func(user *mariadbv1alpha1.User) error{
		validatePassword,
		validateUserCleanupPolicy,
		validateRequire,
	}
	for _, fn := range validateFns {
		if err := fn(user); err != nil {
			return nil, err
		}
	}
	return nil, nil
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type User.
func (v *UserCustomValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	user, ok := newObj.(*mariadbv1alpha1.User)
	if !ok {
		return nil, fmt.Errorf("expected a User object for the newObj but got %T", newObj)
	}
	oldUser, ok := oldObj.(*mariadbv1alpha1.User)
	if !ok {
		return nil, fmt.Errorf("expected a User object for the newObj but got %T", newObj)
	}
	userlog.V(1).Info("Validation for User upon update", "name", user.GetName())

	if err := inmutableWebhook.ValidateUpdate(user, oldUser); err != nil {
		return nil, err
	}
	validateFns := []func(user *mariadbv1alpha1.User) error{
		validatePassword,
		validateUserCleanupPolicy,
		validateRequire,
	}
	for _, fn := range validateFns {
		if err := fn(user); err != nil {
			return nil, err
		}
	}
	return nil, nil
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type User.
func (v *UserCustomValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

func validatePasswordPlugin(user *mariadbv1alpha1.User) error {
	if user.Spec.PasswordPlugin.PluginArgSecretKeyRef != nil && user.Spec.PasswordPlugin.PluginNameSecretKeyRef == nil {
		return field.Invalid(
			field.NewPath("spec").Child("passwordPlugin").Child("pluginArgSecretKeyRef"),
			user.Spec.PasswordPlugin.PluginArgSecretKeyRef,
			"'spec.passwordPlugin.pluginArgSecretKeyRef' can only be set when 'spec.passwordPlugin.pluginNameSecretKeyRef' is set",
		)

	}
	return nil
}

func validatePassword(user *mariadbv1alpha1.User) error {
	if err := validatePasswordPlugin(user); err != nil {
		return err
	}

	definedPasswordMethods := []bool{
		user.Spec.PasswordSecretKeyRef != nil,
		user.Spec.PasswordHashSecretKeyRef != nil,
		user.Spec.PasswordPlugin.PluginNameSecretKeyRef != nil,
	}
	definedPasswordMethods = slices.DeleteFunc(definedPasswordMethods, func(v bool) bool { return !v })

	if len(definedPasswordMethods) > 1 {
		return field.Invalid(
			field.NewPath("spec").Child("passwordSecretKeyRef"),
			user.Spec.PasswordSecretKeyRef,
			"Only one of 'spec.passwordSecretKeyRef', 'spec.passwordHashSecretKeyRef', or "+
				"'spec.passwordPlugin.pluginNameSecretKeyRef' can be defined",
		)
	}
	return nil
}

func validateUserCleanupPolicy(user *mariadbv1alpha1.User) error {
	if user.Spec.CleanupPolicy != nil {
		if err := user.Spec.CleanupPolicy.Validate(); err != nil {
			return field.Invalid(
				field.NewPath("spec").Child("cleanupPolicy"),
				user.Spec.CleanupPolicy,
				err.Error(),
			)
		}
	}
	return nil
}

func validateRequire(user *mariadbv1alpha1.User) error {
	if require := user.Spec.Require; require != nil {
		if err := require.Validate(); err != nil {
			return field.Invalid(
				field.NewPath("spec").Child("require"),
				user.Spec.Require,
				err.Error(),
			)
		}
	}
	return nil
}
