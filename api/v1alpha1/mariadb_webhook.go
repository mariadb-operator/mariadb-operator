package v1alpha1

import (
	"errors"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	log "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var mariadbLogger = log.Log.WithName("mariadb")

func (r *MariaDB) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

//+kubebuilder:webhook:path=/mutate-mariadb-mmontes-io-v1alpha1-mariadb,mutating=true,failurePolicy=fail,sideEffects=None,groups=mariadb.mmontes.io,resources=mariadbs,verbs=create;update,versions=v1alpha1,name=mmariadb.kb.io,admissionReviewVersions=v1

var _ webhook.Defaulter = &MariaDB{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *MariaDB) Default() {
	if r.Spec.Replication != nil && r.Spec.Replication.Enabled {
		mariadbLogger.V(1).Info("Defaulting spec.replication", "mariadb", r.Name)
		r.Spec.Replication.FillWithDefaults()
		return
	}
	if r.Spec.Galera != nil && r.Spec.Galera.Enabled {
		mariadbLogger.V(1).Info("Defaulting spec.galera", "mariadb", r.Name)
		r.Spec.Galera.FillWithDefaults()
		return
	}
}

//+kubebuilder:webhook:path=/validate-mariadb-mmontes-io-v1alpha1-mariadb,mutating=false,failurePolicy=fail,sideEffects=None,groups=mariadb.mmontes.io,resources=mariadbs,verbs=create;update,versions=v1alpha1,name=vmariadb.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &MariaDB{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *MariaDB) ValidateCreate() (admission.Warnings, error) {
	mariadbLogger.V(1).Info("Validate create", "name", r.Name)
	if err := r.validate(); err != nil {
		return nil, err
	}
	return nil, r.validateMaxScale()
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *MariaDB) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	mariadbLogger.V(1).Info("Validate update", "name", r.Name)
	oldMariadb := old.(*MariaDB)
	if err := inmutableWebhook.ValidateUpdate(r, oldMariadb); err != nil {
		return nil, err
	}
	if err := r.validate(); err != nil {
		return nil, err
	}
	return nil, r.validatePrimarySwitchover(oldMariadb)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *MariaDB) ValidateDelete() (admission.Warnings, error) {
	return nil, nil
}

func (r *MariaDB) validate() error {
	validateFns := []func() error{
		r.validateHA,
		r.validateGalera,
		r.validateReplication,
		r.validateBootstrapFrom,
		r.validatePodDisruptionBudget,
		r.validateStorage,
		r.validateRootPassword,
	}
	for _, fn := range validateFns {
		if err := fn(); err != nil {
			return err
		}
	}
	return nil
}

func (r *MariaDB) validateHA() error {
	if r.Replication().Enabled && r.Galera().Enabled {
		return errors.New("You may only enable one HA method at a time, either 'spec.replication' or 'spec.galera'")
	}
	if !r.IsHAEnabled() && r.Spec.Replicas > 1 {
		return field.Invalid(
			field.NewPath("spec").Child("replicas"),
			r.Spec.Replicas,
			"Multiple replicas can only be specified when 'spec.replication' or 'spec.galera' are configured",
		)
	}
	if r.IsHAEnabled() && r.Spec.Replicas <= 1 {
		return field.Invalid(
			field.NewPath("spec").Child("replicas"),
			r.Spec.Replicas,
			"Multiple replicas must be specified when 'spec.replication' or 'spec.galera' are configured",
		)
	}
	return nil
}

func (r *MariaDB) validateMaxScale() error {
	if r.Spec.MaxScaleRef != nil && r.Spec.MaxScale != nil {
		return field.Invalid(
			field.NewPath("spec").Child("maxScaleRef"),
			r.Spec.MaxScaleRef,
			"'spec.maxScaleRef' and 'spec.maxScale' cannot be specified simultaneously",
		)
	}
	return nil
}

func (r *MariaDB) validateGalera() error {
	if !r.Galera().Enabled {
		return nil
	}
	if *r.Galera().Primary.PodIndex < 0 || *r.Galera().Primary.PodIndex >= int(r.Spec.Replicas) {
		return field.Invalid(
			field.NewPath("spec").Child("galera").Child("primary").Child("podIndex"),
			r.Replication().Primary.PodIndex,
			"'spec.galera.primary.podIndex' out of 'spec.replicas' bounds",
		)
	}
	if err := r.Galera().SST.Validate(); err != nil {
		return field.Invalid(
			field.NewPath("spec").Child("galera").Child("sst"),
			r.Galera().SST,
			err.Error(),
		)
	}
	if *r.Galera().ReplicaThreads < 1 {
		return field.Invalid(
			field.NewPath("spec").Child("galera").Child("replicaThreads"),
			r.Galera().ReplicaThreads,
			"'spec.galera.replicaThreads' must be at least 1",
		)
	}
	return nil
}

func (r *MariaDB) validateReplication() error {
	if !r.Replication().Enabled {
		return nil
	}
	if *r.Replication().Primary.PodIndex < 0 || *r.Replication().Primary.PodIndex >= int(r.Spec.Replicas) {
		return field.Invalid(
			field.NewPath("spec").Child("replication").Child("primary").Child("podIndex"),
			r.Replication().Primary.PodIndex,
			"'spec.replication.primary.podIndex' out of 'spec.replicas' bounds",
		)
	}
	if err := r.Replication().Replica.Validate(); err != nil {
		return field.Invalid(
			field.NewPath("spec").Child("replication").Child("replica"),
			r.Replication(),
			err.Error(),
		)
	}
	return nil
}

func (r *MariaDB) validatePrimarySwitchover(old *MariaDB) error {
	if old.Replication().Enabled && old.IsSwitchingPrimary() {
		if *old.Replication().Primary.PodIndex != *r.Replication().Primary.PodIndex {
			return field.Invalid(
				field.NewPath("spec").Child("replication").Child("primary").Child("podIndex"),
				r.Replication().Primary.PodIndex,
				"'spec.replication.primary.podIndex' cannot be updated during a primary switchover",
			)
		}
		if *old.Replication().Primary.AutomaticFailover != *r.Replication().Primary.AutomaticFailover &&
			*r.Replication().Primary.AutomaticFailover {
			return field.Invalid(
				field.NewPath("spec").Child("replication").Child("primary").Child("automaticFailover"),
				r.Replication().Primary.PodIndex,
				"'spec.replication.primary.automaticFailover' cannot be enabled during a primary switchover",
			)
		}
	}
	return nil
}

func (r *MariaDB) validateBootstrapFrom() error {
	if r.Spec.BootstrapFrom == nil {
		return nil
	}
	if err := r.Spec.BootstrapFrom.Validate(); err != nil {
		return field.Invalid(
			field.NewPath("spec").Child("bootstrapFrom"),
			r.Spec.BootstrapFrom,
			err.Error(),
		)
	}
	return nil
}

func (r *MariaDB) validatePodDisruptionBudget() error {
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

func (r *MariaDB) validateStorage() error {
	// spec.ephemeralStorage or spec.volumeClaimTemplate shall be defined explicitly
	if !r.IsEphemeralStorageEnabled() && !r.IsVolumeClaimTemplateDefined() {
		return field.Invalid(
			field.NewPath("spec").Child("volumeClaimTemplate"),
			r.Spec.VolumeClaimTemplate,
			"'spec.ephemeralStorage' or 'spec.volumeClaimTemplate' must be defined",
		)
	}
	// spec.ephemeralStorage and spec.volumeClaimTemplate shall be mutually exclusive
	if r.IsEphemeralStorageEnabled() && r.IsVolumeClaimTemplateDefined() {
		return field.Invalid(
			field.NewPath("spec").Child("ephemeralStorage"),
			r.Spec.EphemeralStorage,
			"'spec.ephemeralStorage' must be disabled when 'spec.volumeClaimTemplate' is specified",
		)
	}

	return nil
}

func (r *MariaDB) validateRootPassword() error {
	if r.IsRootPasswordEmpty() && r.IsRootPasswordDefined() {
		return field.Invalid(
			field.NewPath("spec").Child("rootEmptyPassword"),
			r.Spec.RootEmptyPassword,
			"'spec.rootEmptyPassword' must be disabled when 'spec.rootPasswordSecretKeyRef' is specified",
		)
	}
	return nil
}
