package v1alpha1

import (
	"context"
	"fmt"

	"github.com/mariadb-operator/mariadb-operator/api/mariadb/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// log is for logging in this package.
var maxscalelog = logf.Log.WithName("maxscale-resource")

// SetupMaxScaleWebhookWithManager registers the webhook for MaxScale in the manager.
func SetupMaxScaleWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).For(&v1alpha1.MaxScale{}).
		WithValidator(&MaxScaleCustomValidator{}).
		Complete()
}

// +kubebuilder:webhook:path=/validate-k8s-mariadb-com-v1alpha1-maxscale,mutating=false,failurePolicy=fail,sideEffects=None,groups=k8s.mariadb.com,resources=maxscales,verbs=create;update,versions=v1alpha1,name=vmaxscale-v1alpha1.kb.io,admissionReviewVersions=v1

// MaxScaleCustomValidator struct is responsible for validating the MaxScale resource
// when it is created, updated, or deleted.
type MaxScaleCustomValidator struct{}

var _ webhook.CustomValidator = &MaxScaleCustomValidator{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type MaxScale.
func (v *MaxScaleCustomValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	maxscale, ok := obj.(*v1alpha1.MaxScale)
	if !ok {
		return nil, fmt.Errorf("expected a MaxScale object but got %T", obj)
	}
	maxscalelog.V(1).Info("Validation for MaxScale upon creation", "name", maxscale.GetName())

	validateFns := []func(*v1alpha1.MaxScale) error{
		validateAuth,
		validateCreateServerSources,
		validateServers,
		validateMonitor,
		validateServices,
		validateMaxScalePodDisruptionBudget,
		validateMaxScaleTLS,
	}
	for _, fn := range validateFns {
		if err := fn(maxscale); err != nil {
			return nil, err
		}
	}
	return nil, nil
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type MaxScale.
func (v *MaxScaleCustomValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	maxscale, ok := newObj.(*v1alpha1.MaxScale)
	if !ok {
		return nil, fmt.Errorf("expected a MaxScale object for the newObj but got %T", newObj)
	}
	oldMaxscale, ok := oldObj.(*v1alpha1.MaxScale)
	if !ok {
		return nil, fmt.Errorf("expected a MaxScale object for the newObj but got %T", newObj)
	}
	maxscalelog.V(1).Info("Validation for MaxScale upon update", "name", maxscale.GetName())

	if err := inmutableWebhook.ValidateUpdate(maxscale, oldMaxscale); err != nil {
		return nil, err
	}

	validateFns := []func(*v1alpha1.MaxScale) error{
		validateAuth,
		validateServerSources,
		validateServers,
		validateMonitor,
		validateServices,
		validateMaxScalePodDisruptionBudget,
		validateMaxScaleTLS,
	}
	for _, fn := range validateFns {
		if err := fn(maxscale); err != nil {
			return nil, err
		}
	}
	return nil, nil
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type MaxScale.
func (v *MaxScaleCustomValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

func validateAuth(maxscale *v1alpha1.MaxScale) error {
	if ptr.Deref(maxscale.Spec.Auth.Generate, false) && maxscale.Spec.MariaDBRef == nil {
		return field.Invalid(
			field.NewPath("spec").Child("auth").Child("generate").Child("enabled"),
			maxscale.Spec.MariaDBRef,
			"'spec.auth.generate' can only be enabled when 'spec.mariaDbRef' is set",
		)
	}
	return nil
}

func validateCreateServerSources(maxscale *v1alpha1.MaxScale) error {
	if err := validateServerSources(maxscale); err != nil {
		return err
	}
	if maxscale.Spec.MariaDBRef != nil && maxscale.Spec.Servers != nil {
		return field.Invalid(
			field.NewPath("spec").Child("mariaDbRef"),
			maxscale.Spec.MariaDBRef,
			"'spec.mariaDbRef' and 'spec.servers' cannot be specified simultaneously",
		)
	}
	return nil
}

func validateServerSources(maxscale *v1alpha1.MaxScale) error {
	if maxscale.Spec.MariaDBRef == nil && maxscale.Spec.Servers == nil {
		return field.Invalid(
			field.NewPath("spec").Child("mariaDbRef"),
			maxscale.Spec.MariaDBRef,
			"'spec.mariaDbRef' or 'spec.servers' must be defined",
		)
	}
	return nil
}

func validateServers(maxscale *v1alpha1.MaxScale) error {
	idx := maxscale.ServerIndex()
	if len(idx) != len(maxscale.Spec.Servers) {
		return field.Invalid(
			field.NewPath("spec").Child("servers"),
			maxscale.Spec.Servers,
			"server names must be unique",
		)
	}
	addresses := make(map[string]struct{})
	for _, srv := range maxscale.Spec.Servers {
		addresses[srv.Address] = struct{}{}
	}
	if len(addresses) != len(maxscale.Spec.Servers) {
		return field.Invalid(
			field.NewPath("spec").Child("servers"),
			maxscale.Spec.Servers,
			"server addresses must be unique",
		)
	}
	return nil
}

func validateMonitor(maxscale *v1alpha1.MaxScale) error {
	if maxscale.Spec.MariaDBRef == nil && maxscale.Spec.Monitor.Module == "" {
		return field.Invalid(
			field.NewPath("spec").Child("monitor").Child("module"),
			maxscale.Spec.Monitor.Module,
			"'spec.monitor.module' must be provided when 'spec.mariaDbRef' is not defined",
		)
	}
	if maxscale.Spec.Monitor.Module != "" {
		if err := maxscale.Spec.Monitor.Module.Validate(); err != nil {
			return field.Invalid(
				field.NewPath("spec").Child("monitor").Child("module"),
				maxscale.Spec.Monitor.Module,
				err.Error(),
			)
		}
	}
	return nil
}

func validateServices(maxscale *v1alpha1.MaxScale) error {
	idx := maxscale.ServiceIndex()
	if len(idx) != len(maxscale.Spec.Services) {
		return field.Invalid(
			field.NewPath("spec").Child("services"),
			maxscale.Spec.Services,
			"service names must be unique",
		)
	}
	ports := make(map[int]struct{})
	for _, svc := range maxscale.Spec.Services {
		ports[int(svc.Listener.Port)] = struct{}{}
	}
	if len(ports) != len(maxscale.Spec.Services) {
		return field.Invalid(
			field.NewPath("spec").Child("services"),
			maxscale.Spec.Services,
			"service listener ports must be unique",
		)
	}
	return nil
}

func validateMaxScalePodDisruptionBudget(maxscale *v1alpha1.MaxScale) error {
	if maxscale.Spec.PodDisruptionBudget == nil {
		return nil
	}
	if err := maxscale.Spec.PodDisruptionBudget.Validate(); err != nil {
		return field.Invalid(
			field.NewPath("spec").Child("podDisruptionBudget"),
			maxscale.Spec.PodDisruptionBudget,
			err.Error(),
		)
	}
	return nil
}

func validateMaxScaleTLS(maxscale *v1alpha1.MaxScale) error {
	tls := ptr.Deref(maxscale.Spec.TLS, v1alpha1.MaxScaleTLS{})
	if !tls.Enabled {
		return nil
	}
	validationItems := []tlsValidationItem{
		{
			tlsValue:            maxscale.Spec.TLS,
			caSecretRef:         tls.AdminCASecretRef,
			caFieldPath:         "spec.tls.adminCASecretRef",
			certSecretRef:       tls.AdminCertSecretRef,
			certFieldPath:       "spec.tls.adminCertSecretRef",
			certIssuerRef:       tls.AdminCertIssuerRef,
			certIssuerFieldPath: "spec.tls.adminCertIssuerRef",
		},
		{
			tlsValue:            maxscale.Spec.TLS,
			caSecretRef:         tls.ListenerCASecretRef,
			caFieldPath:         "spec.tls.listenerCASecretRef",
			certSecretRef:       tls.ListenerCertSecretRef,
			certFieldPath:       "spec.tls.listenerCertSecretRef",
			certIssuerRef:       tls.ListenerCertIssuerRef,
			certIssuerFieldPath: "spec.tls.listenerCertIssuerRef",
		},
		{
			tlsValue:      maxscale.Spec.TLS,
			caSecretRef:   tls.ServerCASecretRef,
			caFieldPath:   "spec.tls.serverCASecretRef",
			certSecretRef: tls.ServerCertSecretRef,
			certFieldPath: "spec.tls.serverCertSecretRef",
		},
	}
	for _, item := range validationItems {
		if err := validateTLSCert(&item); err != nil {
			return err
		}
	}
	return nil
}
