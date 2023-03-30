package replication

import (
	"context"
	"fmt"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/pkg/builder"
	labels "github.com/mariadb-operator/mariadb-operator/pkg/builder/labels"
	replConfig "github.com/mariadb-operator/mariadb-operator/pkg/controller/replication/config"
	"github.com/mariadb-operator/mariadb-operator/pkg/refresolver"
	"github.com/sethvargo/go-password/password"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	PasswordSecretKey = "password"

	ReplUser     = "repl"
	PrimaryUser  = "primary"
	ReadonlyUser = "readonly"

	PrimaryCnfKey = "primary.cnf"
	PrimarySqlKey = "primary.sql"

	ReplicaCnfKey = "replica.cnf"
	ReplicaSqlKey = "replica.sql"

	InitShKey = "init.sh"
)

type ReplicationReconciler struct {
	client.Client
	Builder     *builder.Builder
	RefResolver *refresolver.RefResolver
}

func NewReplicationReconciler(client client.Client, builder *builder.Builder,
	refResolver *refresolver.RefResolver) *ReplicationReconciler {
	return &ReplicationReconciler{
		Client:      client,
		Builder:     builder,
		RefResolver: refResolver,
	}
}

type reconcilePhase struct {
	resource  string
	reconcile func(context.Context, *mariadbv1alpha1.MariaDB, types.NamespacedName) error
	key       types.NamespacedName
}

type reconcileResult struct {
	sql string
	cnf string
}

func (r *ReplicationReconciler) Reconcile(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	mariaDbKey types.NamespacedName) error {
	if mariadb.Spec.Replication == nil {
		return nil
	}

	phases := []reconcilePhase{
		{
			resource:  "Repl Secret",
			reconcile: r.reconcilePasswordSecret,
			key:       replPasswordKey(mariadb),
		},
		{
			resource:  "Config Secret",
			reconcile: r.reconcileConfigSecret,
			key:       replConfig.ConfigReplicaKey(mariadb),
		},
		{
			resource:  "PodDisruptionBudget",
			reconcile: r.reconcilePodDisruptionBudget,
			key:       PodDisruptionBudgetKey(mariadb),
		},
		{
			resource:  "Primary Service",
			reconcile: r.reconcilePrimaryService,
			key:       PrimaryServiceKey(mariadb),
		},
	}

	for _, p := range phases {
		if err := p.reconcile(ctx, mariadb, p.key); err != nil {
			return fmt.Errorf("error reconciling %s: %v", p.resource, err)
		}
	}
	return nil
}

func (r *ReplicationReconciler) reconcilePasswordSecret(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	key types.NamespacedName) error {
	var existingSecret corev1.Secret
	if err := r.Get(ctx, key, &existingSecret); err == nil {
		return nil
	}

	password, err := password.Generate(16, 4, 0, false, false)
	if err != nil {
		return fmt.Errorf("error generating password: %v", err)
	}

	opts := builder.SecretOpts{
		Key: key,
		Data: map[string][]byte{
			PasswordSecretKey: []byte(password),
		},
		Labels: labels.NewLabelsBuilder().WithMariaDB(mariadb).Build(),
	}
	secret, err := r.Builder.BuildSecret(opts, mariadb)
	if err != nil {
		return fmt.Errorf("error building password Secret: %v", err)
	}

	if err := r.Create(ctx, secret); err != nil {
		return fmt.Errorf("error creating password Secret: %v", err)
	}
	return nil
}

func (r *ReplicationReconciler) reconcilePodDisruptionBudget(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	mariaDbKey types.NamespacedName) error {

	if mariadb.Spec.PodDisruptionBudget != nil {
		return nil
	}

	key := PodDisruptionBudgetKey(mariadb)
	var existingPDB policyv1.PodDisruptionBudget
	if err := r.Get(ctx, key, &existingPDB); err == nil {
		return nil
	}

	selectorLabels :=
		labels.NewLabelsBuilder().
			WithMariaDB(mariadb).
			Build()
	minAvailable := intstr.FromString("50%")
	opts := builder.PodDisruptionBudgetOpts{
		Key:            key,
		MinAvailable:   &minAvailable,
		SelectorLabels: selectorLabels,
	}
	pdb, err := r.Builder.BuildPodDisruptionBudget(&opts, mariadb)
	if err != nil {
		return fmt.Errorf("error building PodDisruptionBudget: %v", err)
	}

	if err := r.Create(ctx, pdb); err != nil {
		return fmt.Errorf("error creating PodDisruptionBudget: %v", err)
	}
	return nil
}

func (r *ReplicationReconciler) reconcileConfigSecret(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	key types.NamespacedName) error {
	var existingSecret corev1.Secret
	if err := r.Get(ctx, key, &existingSecret); err == nil {
		return nil
	}

	primaryResult, err := r.reconcilePrimary(ctx, mariadb)
	if err != nil {
		return fmt.Errorf("error reconciling primary: %v", err)
	}
	replicaResult, err := r.reconcileReplica(ctx, mariadb)
	if err != nil {
		return fmt.Errorf("error reconciling replica: %v", err)
	}
	initOpts := replConfig.InitShOpts{
		PrimaryCnf: PrimaryCnfKey,
		PrimarySql: PrimarySqlKey,
		ReplicaCnf: ReplicaCnfKey,
		ReplicaSql: ReplicaSqlKey,
	}
	initScript, err := replConfig.InitSh(initOpts)
	if err != nil {
		return fmt.Errorf("error generating init.sh: %v", err)
	}

	secretOps := builder.SecretOpts{
		Key: key,
		Data: map[string][]byte{
			PrimaryCnfKey: []byte(primaryResult.cnf),
			PrimarySqlKey: []byte(primaryResult.sql),
			ReplicaCnfKey: []byte(replicaResult.cnf),
			ReplicaSqlKey: []byte(replicaResult.sql),
			InitShKey:     []byte(initScript),
		},
		Labels: labels.NewLabelsBuilder().WithMariaDB(mariadb).Build(),
	}
	secret, err := r.Builder.BuildSecret(secretOps, mariadb)
	if err != nil {
		return fmt.Errorf("error building configuration Secret: %v", err)
	}

	if err := r.Create(ctx, secret); err != nil {
		return fmt.Errorf("error creating configuration Secret: %v", err)
	}
	return nil
}

func (r *ReplicationReconciler) reconcilePrimary(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) (*reconcileResult, error) {
	var replSecret corev1.Secret
	if err := r.Get(ctx, replPasswordKey(mariadb), &replSecret); err != nil {
		return nil, fmt.Errorf("error getting replication password Secret: %v", err)
	}

	cnf, err := replConfig.PrimaryCnf(*mariadb.Spec.Replication)
	if err != nil {
		return nil, fmt.Errorf("error generating primary.cnf: %v", err)
	}

	users := []replConfig.PrimarySqlUser{}
	if mariadb.Spec.Username != nil && mariadb.Spec.PasswordSecretKeyRef != nil {
		password, err := r.RefResolver.SecretKeyRef(ctx, *mariadb.Spec.PasswordSecretKeyRef, mariadb.Namespace)
		if err != nil {
			return nil, fmt.Errorf("error getting default user password: %v", err)
		}
		users = append(users, replConfig.PrimarySqlUser{
			Username: *mariadb.Spec.Username,
			Password: password,
		})
	}

	databases := []string{}
	if mariadb.Spec.Database != nil {
		databases = append(databases, *mariadb.Spec.Database)
	}

	opts := replConfig.PrimarySqlOpts{
		ReplUser:     ReplUser,
		ReplPassword: string(replSecret.Data[PasswordSecretKey]),
		Users:        users,
		Databases:    databases,
	}
	sql, err := replConfig.PrimarySql(opts)
	if err != nil {
		return nil, fmt.Errorf("error generating primary.sql: %v", err)
	}

	return &reconcileResult{
		sql: sql,
		cnf: cnf,
	}, nil
}

func (r *ReplicationReconciler) reconcileReplica(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) (*reconcileResult, error) {
	var replSecret corev1.Secret
	if err := r.Get(ctx, replPasswordKey(mariadb), &replSecret); err != nil {
		return nil, fmt.Errorf("error getting replication password Secret: %v", err)
	}

	cnf, err := replConfig.ReplicaCnf(*mariadb.Spec.Replication)
	if err != nil {
		return nil, fmt.Errorf("error generating replica.cnf: %v", err)
	}
	replicaOpts := replConfig.ReplicaSqlOpts{
		Meta:     mariadb.ObjectMeta,
		User:     ReplUser,
		Password: string(replSecret.Data[PasswordSecretKey]),
		Retries:  mariadb.Spec.Replication.ReplicaRetries,
	}
	sql, err := replConfig.ReplicaSql(replicaOpts)
	if err != nil {
		return nil, fmt.Errorf("error generating replica.sql: %v", err)
	}

	return &reconcileResult{
		sql: sql,
		cnf: cnf,
	}, nil
}

func (r *ReplicationReconciler) reconcilePrimaryService(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	key types.NamespacedName) error {
	serviceLabels :=
		labels.NewLabelsBuilder().
			WithMariaDB(mariadb).
			WithStatefulSetPod(mariadb, 0).
			Build()
	desiredSvc, err := r.Builder.BuildService(mariadb, key, serviceLabels)
	if err != nil {
		return fmt.Errorf("error building Service: %v", err)
	}

	var existingSvc corev1.Service
	if err := r.Get(ctx, key, &existingSvc); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("error getting Service: %v", err)
		}
		if err := r.Create(ctx, desiredSvc); err != nil {
			return fmt.Errorf("error creating Service: %v", err)
		}
		return nil
	}

	patch := client.MergeFrom(existingSvc.DeepCopy())
	existingSvc.Spec.Ports = desiredSvc.Spec.Ports

	if err := r.Patch(ctx, &existingSvc, patch); err != nil {
		return fmt.Errorf("error patching Service: %v", err)
	}
	return nil
}

func PodDisruptionBudgetKey(mariadb *mariadbv1alpha1.MariaDB) types.NamespacedName {
	return types.NamespacedName{
		Name:      mariadb.Name,
		Namespace: mariadb.Namespace,
	}
}

func PrimaryServiceKey(mariadb *mariadbv1alpha1.MariaDB) types.NamespacedName {
	return types.NamespacedName{
		Name:      fmt.Sprintf("primary-%s", mariadb.Name),
		Namespace: mariadb.Namespace,
	}
}

func replPasswordKey(mariadb *mariadbv1alpha1.MariaDB) types.NamespacedName {
	return types.NamespacedName{
		Name:      fmt.Sprintf("repl-password-%s", mariadb.Name),
		Namespace: mariadb.Namespace,
	}
}
