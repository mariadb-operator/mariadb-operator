package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"
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

//+kubebuilder:webhook:path=/validate-k8s-mariadb-com-v1alpha1-maxscale,mutating=false,failurePolicy=fail,sideEffects=None,groups=k8s.mariadb.com,resources=maxscales,verbs=create;update,versions=v1alpha1,name=vmaxscale.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &MaxScale{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *MaxScale) ValidateCreate() (admission.Warnings, error) {
	maxscaleLogger.V(1).Info("Validate create", "name", r.Name)
	validateFns := []func() error{
		r.validateAuth,
		r.validateCreateServerSources,
		r.validateServers,
		r.validateMonitor,
		r.validateServices,
		r.validatePodDisruptionBudget,
	}
	for _, fn := range validateFns {
		if err := fn(); err != nil {
			return nil, err
		}
	}
	return nil, nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *MaxScale) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	maxscaleLogger.V(1).Info("Validate update", "name", r.Name)
	oldMaxScale := old.(*MaxScale)
	if err := inmutableWebhook.ValidateUpdate(r, oldMaxScale); err != nil {
		return nil, err
	}
	validateFns := []func() error{
		r.validateAuth,
		r.validateServerSources,
		r.validateServers,
		r.validateMonitor,
		r.validateServices,
		r.validatePodDisruptionBudget,
	}
	for _, fn := range validateFns {
		if err := fn(); err != nil {
			return nil, err
		}
	}
	return nil, nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *MaxScale) ValidateDelete() (admission.Warnings, error) {
	return nil, nil
}

func (r *MaxScale) validateAuth() error {
	if ptr.Deref(r.Spec.Auth.Generate, false) && r.Spec.MariaDBRef == nil {
		return field.Invalid(
			field.NewPath("spec").Child("auth").Child("generate").Child("enabled"),
			r.Spec.MariaDBRef,
			"'spec.auth.generate' can only be enabled when 'spec.mariaDbRef' is set",
		)
	}
	return nil
}

func (r *MaxScale) validateCreateServerSources() error {
	if err := r.validateServerSources(); err != nil {
		return err
	}
	if r.Spec.MariaDBRef != nil && r.Spec.Servers != nil {
		return field.Invalid(
			field.NewPath("spec").Child("mariaDbRef"),
			r.Spec.MariaDBRef,
			"'spec.mariaDbRef' and 'spec.servers' cannot be specified simultaneously",
		)
	}
	return nil
}

func (r *MaxScale) validateServerSources() error {
	if r.Spec.MariaDBRef == nil && r.Spec.Servers == nil {
		return field.Invalid(
			field.NewPath("spec").Child("mariaDbRef"),
			r.Spec.MariaDBRef,
			"'spec.mariaDbRef' or 'spec.servers' must be defined",
		)
	}
	return nil
}

func (r *MaxScale) validateServers() error {
	idx := r.ServerIndex()
	if len(idx) != len(r.Spec.Servers) {
		return field.Invalid(
			field.NewPath("spec").Child("servers"),
			r.Spec.Servers,
			"server names must be unique",
		)
	}
	addresses := make(map[string]struct{})
	for _, srv := range r.Spec.Servers {
		addresses[srv.Address] = struct{}{}
	}
	if len(addresses) != len(r.Spec.Servers) {
		return field.Invalid(
			field.NewPath("spec").Child("servers"),
			r.Spec.Servers,
			"server addresses must be unique",
		)
	}
	return nil
}

func (r *MaxScale) validateMonitor() error {
	if r.Spec.MariaDBRef == nil && r.Spec.Monitor.Module == "" {
		return field.Invalid(
			field.NewPath("spec").Child("monitor").Child("module"),
			r.Spec.Monitor.Module,
			"'spec.monitor.module' must be provided when 'spec.mariaDbRef' is not defined",
		)
	}
	if r.Spec.Monitor.Module != "" {
		if err := r.Spec.Monitor.Module.Validate(); err != nil {
			return field.Invalid(
				field.NewPath("spec").Child("monitor").Child("module"),
				r.Spec.Monitor.Module,
				err.Error(),
			)
		}
	}
	return nil
}

func (r *MaxScale) validateServices() error {
	idx := r.ServiceIndex()
	if len(idx) != len(r.Spec.Services) {
		return field.Invalid(
			field.NewPath("spec").Child("services"),
			r.Spec.Services,
			"service names must be unique",
		)
	}
	ports := make(map[int]struct{})
	for _, svc := range r.Spec.Services {
		ports[int(svc.Listener.Port)] = struct{}{}
	}
	if len(ports) != len(r.Spec.Services) {
		return field.Invalid(
			field.NewPath("spec").Child("services"),
			r.Spec.Services,
			"service listener ports must be unique",
		)
	}
	return nil
}

func (r *MaxScale) validatePodDisruptionBudget() error {
	if r.Spec.PodDisruptionBudget == nil {
		return nil
	}
	if err := r.Spec.PodDisruptionBudget.Validate(); err != nil {
		return field.Invalid(
			field.NewPath("spec").Child("podDisruptionBudget"),
			r.Spec.PodDisruptionBudget,
			err.Error(),
		)
	}
	return nil
}
