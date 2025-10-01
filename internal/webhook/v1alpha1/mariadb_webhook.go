package v1alpha1

import (
	"context"
	"errors"
	"fmt"
	"reflect"

	"github.com/mariadb-operator/mariadb-operator/v25/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/environment"
	galerakeys "github.com/mariadb-operator/mariadb-operator/v25/pkg/galera/config/keys"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// log is for logging in this package.
var mariadblog = logf.Log.WithName("mariadb-resource")

// SetupMariaDBWebhookWithManager registers the webhook for MariaDB in the manager.
func SetupMariaDBWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).For(&v1alpha1.MariaDB{}).
		WithValidator(&MariaDBCustomValidator{}).
		WithDefaulter(&MariaDBCustomDefaulter{}).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-k8s-mariadb-com-v1alpha1-mariadb,mutating=true,failurePolicy=fail,sideEffects=None,groups=k8s.mariadb.com,resources=mariadbs,verbs=create;update,versions=v1alpha1,name=mmariadb-v1alpha1.kb.io,admissionReviewVersions=v1

// MariaDBCustomDefaulter struct is responsible for setting default values on the custom resource of the
// Kind MariaDB when those are created or updated.
type MariaDBCustomDefaulter struct{}

var _ webhook.CustomDefaulter = &MariaDBCustomDefaulter{}

// Default implements webhook.CustomDefaulter so a webhook will be registered for the Kind MariaDB.
func (d *MariaDBCustomDefaulter) Default(ctx context.Context, obj runtime.Object) error {
	mariadb, ok := obj.(*v1alpha1.MariaDB)
	if !ok {
		return fmt.Errorf("expected an MariaDB object but got %T", obj)
	}
	mariadblog.V(1).Info("Defaulting for MariaDB", "name", mariadb.GetName())

	env, err := environment.GetOperatorEnv(ctx)
	if err != nil {
		return fmt.Errorf("error getting the environment: %v", err)
	}

	if mariadb.IsReplicationEnabled() {
		mariadblog.V(1).Info("Defaulting spec.replication", "mariadb", mariadb.Name)
		return mariadb.Spec.Replication.SetDefaults(mariadb, env)
	}
	return nil
}

// +kubebuilder:webhook:path=/validate-k8s-mariadb-com-v1alpha1-mariadb,mutating=false,failurePolicy=fail,sideEffects=None,groups=k8s.mariadb.com,resources=mariadbs,verbs=create;update,versions=v1alpha1,name=vmariadb-v1alpha1.kb.io,admissionReviewVersions=v1

// MariaDBCustomValidator struct is responsible for validating the MariaDB resource
// when it is created, updated, or deleted.
type MariaDBCustomValidator struct{}

var _ webhook.CustomValidator = &MariaDBCustomValidator{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type MariaDB.
func (v *MariaDBCustomValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	mariadb, ok := obj.(*v1alpha1.MariaDB)
	if !ok {
		return nil, fmt.Errorf("expected a MariaDB object but got %T", obj)
	}
	mariadblog.V(1).Info("Validation for MariaDB upon creation", "name", mariadb.GetName())

	validateFns := []func(*v1alpha1.MariaDB) error{
		validateHA,
		validateGalera,
		validateReplication,
		validateBootstrapFrom,
		validatePodDisruptionBudget,
		validateStorage,
		validateRootPassword,
		validateMaxScale,
		validateTLS,
	}
	for _, fn := range validateFns {
		if err := fn(mariadb); err != nil {
			return nil, err
		}
	}
	return nil, nil
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type MariaDB.
func (v *MariaDBCustomValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	mariadb, ok := newObj.(*v1alpha1.MariaDB)
	if !ok {
		return nil, fmt.Errorf("expected a MariaDB object for the newObj but got %T", newObj)
	}
	oldMariadb, ok := oldObj.(*v1alpha1.MariaDB)
	if !ok {
		return nil, fmt.Errorf("expected a MariaDB object for the newObj but got %T", newObj)

	}
	mariadblog.V(1).Info("Validation for MariaDB upon update", "name", mariadb.GetName())

	if err := inmutableWebhook.ValidateUpdate(mariadb, oldMariadb); err != nil {
		return nil, err
	}
	validateFns := []func(*v1alpha1.MariaDB) error{
		validateHA,
		validateGalera,
		validateReplication,
		validateBootstrapFrom,
		validatePodDisruptionBudget,
		validateStorage,
		validateRootPassword,
		validateTLS,
	}
	for _, fn := range validateFns {
		if err := fn(mariadb); err != nil {
			return nil, err
		}
	}

	if err := validatePrimarySwitchover(mariadb, oldMariadb); err != nil {
		return nil, err
	}
	return nil, validateUpdateStorage(mariadb, oldMariadb)
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type MariaDB.
func (v *MariaDBCustomValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

func validateHA(mariadb *v1alpha1.MariaDB) error {
	if mariadb.IsReplicationEnabled() && mariadb.IsGaleraEnabled() {
		return errors.New("you may only enable one HA method at a time, either 'spec.replication' or 'spec.galera'")
	}
	if !mariadb.IsHAEnabled() && mariadb.Spec.Replicas > 1 {
		return field.Invalid(
			field.NewPath("spec").Child("replicas"),
			mariadb.Spec.Replicas,
			"Multiple replicas can only be specified when 'spec.replication' or 'spec.galera' are configured",
		)
	}
	if mariadb.IsHAEnabled() && mariadb.Spec.Replicas <= 1 {
		return field.Invalid(
			field.NewPath("spec").Child("replicas"),
			mariadb.Spec.Replicas,
			"Multiple replicas must be specified when 'spec.replication' or 'spec.galera' are configured",
		)
	}
	return nil
}

func validateMaxScale(mariadb *v1alpha1.MariaDB) error {
	if mariadb.Spec.MaxScaleRef != nil && mariadb.Spec.MaxScale != nil {
		return field.Invalid(
			field.NewPath("spec").Child("maxScaleRef"),
			mariadb.Spec.MaxScaleRef,
			"'spec.maxScaleRef' and 'spec.maxScale' cannot be specified simultaneously",
		)
	}
	return nil
}

func validateGalera(mariadb *v1alpha1.MariaDB) error {
	galera := ptr.Deref(mariadb.Spec.Galera, v1alpha1.Galera{})
	if !galera.Enabled {
		return nil
	}

	if galera.Primary.PodIndex != nil {
		if *galera.Primary.PodIndex < 0 || *galera.Primary.PodIndex >= int(mariadb.Spec.Replicas) {
			return field.Invalid(
				field.NewPath("spec").Child("galera").Child("primary").Child("podIndex"),
				ptr.Deref(mariadb.Spec.Galera, v1alpha1.Galera{}).Primary.PodIndex,
				"'spec.galera.primary.podIndex' out of 'spec.replicas' bounds",
			)
		}
	}

	if !reflect.ValueOf(galera.SST).IsZero() {
		if err := galera.SST.Validate(); err != nil {
			return field.Invalid(
				field.NewPath("spec").Child("galera").Child("sst"),
				galera.SST,
				err.Error(),
			)
		}
	}

	if galera.ReplicaThreads < 0 {
		return field.Invalid(
			field.NewPath("spec").Child("galera").Child("replicaThreads"),
			galera.ReplicaThreads,
			"'spec.galera.replicaThreads' must be at least 1",
		)
	}

	_, exists := galera.ProviderOptions[galerakeys.WsrepOptISTRecvAddr]
	if exists {
		return field.Invalid(
			field.NewPath("spec").Child("galera").Child("providerOptions"),
			galera.ProviderOptions,
			"'spec.galera.providerOptions' cannot contain: ist.recv_addr",
		)
	}

	if err := galera.Agent.Validate(); err != nil {
		return field.Invalid(
			field.NewPath("spec").Child("galera").Child("agent"),
			galera.Agent,
			err.Error(),
		)
	}

	if galera.Recovery != nil {
		if err := galera.Recovery.Validate(mariadb); err != nil {
			return field.Invalid(
				field.NewPath("spec").Child("galera").Child("recovery"),
				galera.Recovery,
				err.Error(),
			)
		}
	}

	return nil
}

func validateReplication(mariadb *v1alpha1.MariaDB) error {
	replication := ptr.Deref(mariadb.Spec.Replication, v1alpha1.Replication{})
	if !replication.Enabled {
		return nil
	}
	if *replication.Primary.PodIndex < 0 || *replication.Primary.PodIndex >= int(mariadb.Spec.Replicas) {
		return field.Invalid(
			field.NewPath("spec").Child("replication").Child("primary").Child("podIndex"),
			replication.Primary.PodIndex,
			"'spec.replication.primary.podIndex' out of 'spec.replicas' bounds",
		)
	}
	if err := replication.Replica.Validate(); err != nil {
		return field.Invalid(
			field.NewPath("spec").Child("replication").Child("replica"),
			replication,
			err.Error(),
		)
	}
	return nil
}

func validatePrimarySwitchover(mariadb, old *v1alpha1.MariaDB) error {
	if old.IsReplicationEnabled() && old.IsSwitchingPrimary() {
		oldReplication := ptr.Deref(old.Spec.Replication, v1alpha1.Replication{})
		mariadbReplication := ptr.Deref(mariadb.Spec.Replication, v1alpha1.Replication{})
		if *oldReplication.Primary.PodIndex != *mariadbReplication.Primary.PodIndex {
			return field.Invalid(
				field.NewPath("spec").Child("replication").Child("primary").Child("podIndex"),
				mariadbReplication.Primary.PodIndex,
				"'spec.replication.primary.podIndex' cannot be updated during a primary switchover",
			)
		}
		if *oldReplication.Primary.AutomaticFailover != *mariadbReplication.Primary.AutomaticFailover &&
			*mariadbReplication.Primary.AutomaticFailover {
			return field.Invalid(
				field.NewPath("spec").Child("replication").Child("primary").Child("automaticFailover"),
				mariadbReplication.Primary.PodIndex,
				"'spec.replication.primary.automaticFailover' cannot be enabled during a primary switchover",
			)
		}
	}
	return nil
}

func validateBootstrapFrom(mariadb *v1alpha1.MariaDB) error {
	if mariadb.Spec.BootstrapFrom == nil {
		return nil
	}
	if err := mariadb.Spec.BootstrapFrom.Validate(); err != nil {
		return field.Invalid(
			field.NewPath("spec").Child("bootstrapFrom"),
			mariadb.Spec.BootstrapFrom,
			err.Error(),
		)
	}
	return nil
}

func validatePodDisruptionBudget(mariadb *v1alpha1.MariaDB) error {
	if mariadb.Spec.PodDisruptionBudget == nil {
		return nil
	}
	if err := mariadb.Spec.PodDisruptionBudget.Validate(); err != nil {
		return field.Invalid(
			field.NewPath("spec").Child("podDisruptionBudget"),
			mariadb.Spec.PodDisruptionBudget,
			err.Error(),
		)
	}
	return nil
}

func validateStorage(mariadb *v1alpha1.MariaDB) error {
	if err := mariadb.Spec.Storage.Validate(mariadb); err != nil {
		return field.Invalid(
			field.NewPath("spec").Child("storage"),
			mariadb.Spec.Storage,
			err.Error(),
		)
	}
	return nil
}

func validateUpdateStorage(mariadb, old *v1alpha1.MariaDB) error {
	if err := validateStorage(mariadb); err != nil {
		return err
	}
	currentSize := mariadb.Spec.Storage.GetSize()
	oldSize := old.Spec.Storage.GetSize()

	if currentSize != nil && oldSize != nil && currentSize.Cmp(*oldSize) < 0 {
		return field.Invalid(
			field.NewPath("spec").Child("storage"),
			mariadb.Spec.Storage,
			"Storage size cannot be decreased",
		)
	}
	return nil
}

func validateRootPassword(mariadb *v1alpha1.MariaDB) error {
	if mariadb.IsRootPasswordEmpty() && mariadb.IsRootPasswordDefined() {
		return field.Invalid(
			field.NewPath("spec").Child("rootEmptyPassword"),
			mariadb.Spec.RootEmptyPassword,
			"'spec.rootEmptyPassword' must be disabled when 'spec.rootPasswordSecretKeyRef' is specified",
		)
	}
	return nil
}

func validateTLS(mariadb *v1alpha1.MariaDB) error {
	tls := ptr.Deref(mariadb.Spec.TLS, v1alpha1.TLS{})
	if !tls.Enabled {
		return nil
	}
	validationItems := []tlsValidationItem{
		{
			tlsValue:            mariadb.Spec.TLS,
			caSecretRef:         tls.ServerCASecretRef,
			caFieldPath:         "spec.tls.serverCASecretRef",
			certSecretRef:       tls.ServerCertSecretRef,
			certFieldPath:       "spec.tls.serverCertSecretRef",
			certIssuerRef:       tls.ServerCertIssuerRef,
			certIssuerFieldPath: "spec.tls.serverCertIssuerRef",
		},
		{
			tlsValue:            mariadb.Spec.TLS,
			caSecretRef:         tls.ClientCASecretRef,
			caFieldPath:         "spec.tls.clientCASecretRef",
			certSecretRef:       tls.ClientCertSecretRef,
			certFieldPath:       "spec.tls.clientCertSecretRef",
			certIssuerRef:       tls.ClientCertIssuerRef,
			certIssuerFieldPath: "spec.tls.clientCertIssuerRef",
		},
	}
	for _, item := range validationItems {
		if err := validateTLSCert(&item); err != nil {
			return err
		}
	}
	return nil
}
