package v1alpha1

import (
	"context"
	"fmt"
	"html/template"
	"time"

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
var connectionlog = logf.Log.WithName("connection-resource")

// SetupConnectionWebhookWithManager registers the webhook for Connection in the manager.
func SetupConnectionWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).For(&k8sv1alpha1.Connection{}).
		WithValidator(&ConnectionCustomValidator{}).
		Complete()
}

// +kubebuilder:webhook:path=/validate-k8s-mariadb-com-v1alpha1-connection,mutating=false,failurePolicy=fail,sideEffects=None,groups=k8s.mariadb.com,resources=connections,verbs=create;update,versions=v1alpha1,name=vconnection-v1alpha1.kb.io,admissionReviewVersions=v1

// ConnectionCustomValidator struct is responsible for validating the Connection resource
// when it is created, updated, or deleted.
type ConnectionCustomValidator struct{}

var _ webhook.CustomValidator = &ConnectionCustomValidator{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type Connection.
func (v *ConnectionCustomValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	connection, ok := obj.(*k8sv1alpha1.Connection)
	if !ok {
		return nil, fmt.Errorf("expected a Connection object but got %T", obj)
	}
	connectionlog.V(1).Info("Validation for Connection upon creation", "name", connection.GetName())

	return validateConnection(connection)
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type Connection.
func (v *ConnectionCustomValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	connection, ok := newObj.(*k8sv1alpha1.Connection)
	if !ok {
		return nil, fmt.Errorf("expected a Connection object for the newObj but got %T", newObj)
	}
	oldConnection, ok := oldObj.(*k8sv1alpha1.Connection)
	if !ok {
		return nil, fmt.Errorf("expected a Connection object for the newObj but got %T", newObj)
	}
	connectionlog.V(1).Info("Validation for Connection upon update", "name", connection.GetName())

	if err := inmutableWebhook.ValidateUpdate(connection, oldConnection); err != nil {
		return nil, err
	}
	return validateConnection(connection)
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type Connection.
func (v *ConnectionCustomValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

func validateConnection(conn *v1alpha1.Connection) (admission.Warnings, error) {
	validateFuncs := []func(*v1alpha1.Connection) error{
		validateRefs,
		validateClientCreds,
		validateHealthCheck,
		validateCustomDSNFormat,
	}
	for _, validateFn := range validateFuncs {
		if err := validateFn(conn); err != nil {
			return nil, err
		}
	}
	return nil, nil
}

func validateRefs(conn *v1alpha1.Connection) error {
	if conn.Spec.MariaDBRef == nil && conn.Spec.MaxScaleRef == nil {
		return field.Invalid(
			field.NewPath("spec").Child("mariaDbRef"),
			conn.Spec.MariaDBRef,
			"'spec.mariaDbRef' or 'spec.maxScaleRef' must be defined",
		)
	}
	if conn.Spec.MariaDBRef != nil && conn.Spec.MaxScaleRef != nil {
		return field.Invalid(
			field.NewPath("spec").Child("mariaDbRef"),
			conn.Spec.MariaDBRef,
			"'spec.mariaDbRef' and 'spec.maxScaleRef' cannot be specified simultaneously",
		)
	}
	return nil
}

func validateClientCreds(conn *v1alpha1.Connection) error {
	if conn.Spec.PasswordSecretKeyRef == nil && conn.Spec.TLSClientCertSecretRef == nil {
		return field.Invalid(
			field.NewPath("spec"),
			conn.Spec,
			"'spec.passwordSecretKeyRef' or 'spec.tlsClientCertSecretRef' must be defined",
		)
	}
	return nil
}

func validateHealthCheck(conn *v1alpha1.Connection) error {
	if conn.Spec.HealthCheck == nil {
		return nil
	}
	if conn.Spec.HealthCheck.Interval != nil {
		duration := conn.Spec.HealthCheck.Interval.Duration.String()
		if _, err := time.ParseDuration(duration); err != nil {
			return field.Invalid(
				field.NewPath("spec").Child("healthCheck").Child("interval"),
				conn.Spec.HealthCheck.Interval,
				fmt.Sprintf("invalid duration: '%s'", duration),
			)
		}
	}
	if conn.Spec.HealthCheck.RetryInterval != nil {
		duration := conn.Spec.HealthCheck.RetryInterval.Duration.String()
		if _, err := time.ParseDuration(duration); err != nil {
			return field.Invalid(
				field.NewPath("spec").Child("healthCheck").Child("retryInterval"),
				conn.Spec.HealthCheck.RetryInterval,
				fmt.Sprintf("invalid duration: '%s'", duration),
			)
		}
	}
	return nil
}

func validateCustomDSNFormat(conn *v1alpha1.Connection) error {
	if conn.Spec.SecretTemplate == nil || conn.Spec.SecretTemplate.Format == nil {
		return nil
	}

	_, err := template.New("").Parse(*conn.Spec.SecretTemplate.Format)
	if err != nil {
		return field.Invalid(
			field.NewPath("spec").Child("secretTemplate").Child("format"),
			conn.Spec.SecretTemplate.Format,
			fmt.Sprintf("invalid format template: '%s'", err),
		)
	}

	return nil
}
