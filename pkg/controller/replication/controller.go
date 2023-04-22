package replication

import (
	"context"
	"fmt"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/pkg/builder"
	labels "github.com/mariadb-operator/mariadb-operator/pkg/builder/labels"
	replresources "github.com/mariadb-operator/mariadb-operator/pkg/controller/replication/resources"
	mariadbclient "github.com/mariadb-operator/mariadb-operator/pkg/mariadb"
	"github.com/mariadb-operator/mariadb-operator/pkg/refresolver"
	"github.com/mariadb-operator/mariadb-operator/pkg/statefulset"
	"github.com/sethvargo/go-password/password"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	PasswordSecretKey = "password"
	ReplUser          = "repl"
	PrimaryUser       = "primary"
	ReadonlyUser      = "readonly"
	ConnectionName    = "mariadb-operator"
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

type reconcileRequest struct {
	mariadb   *mariadbv1alpha1.MariaDB
	key       types.NamespacedName
	clientSet *mariadbClientSet
}

type replicationPhase struct {
	name      string
	key       types.NamespacedName
	reconcile func(context.Context, *reconcileRequest) error
}

func (r *ReplicationReconciler) Reconcile(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	mariaDbKey types.NamespacedName) error {
	if mariadb.Spec.Replication == nil {
		return nil
	}
	if meta.IsStatusConditionFalse(mariadb.Status.Conditions, mariadbv1alpha1.ConditionTypePrimarySwitched) {
		clientSet, err := newMariaDBClientSet(ctx, mariadb, r.RefResolver)
		if err != nil {
			return fmt.Errorf("error creating mariadb clientset: %v", err)
		}
		defer clientSet.close()

		req := reconcileRequest{
			mariadb:   mariadb,
			key:       mariaDbKey,
			clientSet: clientSet,
		}
		if err := r.reconcileSwitchover(ctx, &req); err != nil {
			return fmt.Errorf("error recovering primary switchover: %v", err)
		}
		return nil
	}
	if !mariadb.IsReady() {
		return nil
	}

	clientSet, err := newMariaDBClientSet(ctx, mariadb, r.RefResolver)
	if err != nil {
		return fmt.Errorf("error creating mariadb clientset: %v", err)
	}
	defer clientSet.close()

	phases := []replicationPhase{
		{
			name:      "configure Primary",
			key:       mariaDbKey,
			reconcile: r.configurePrimary,
		},
		{
			name:      "configure Replicas",
			key:       mariaDbKey,
			reconcile: r.configureReplicas,
		},
		{
			name:      "reconcile PodDisruptionBudget",
			key:       replresources.PodDisruptionBudgetKey(mariadb),
			reconcile: r.reconcilePodDisruptionBudget,
		},
		{
			name:      "reconcile primary Service",
			reconcile: r.reconcilePrimaryService,
			key:       replresources.PrimaryServiceKey(mariadb),
		},
		{
			name:      "reconcile primary Connection",
			key:       replresources.PrimaryConnectioneKey(mariadb),
			reconcile: r.reconcilePrimaryConn,
		},
		{
			name:      "update currentPrimaryPodIndex",
			key:       mariaDbKey,
			reconcile: r.updateCurrentPrimaryPodIndex,
		},
		{
			name:      "reconcile switchover",
			key:       mariaDbKey,
			reconcile: r.reconcileSwitchover,
		},
	}

	for _, p := range phases {
		req := reconcileRequest{
			mariadb:   mariadb,
			key:       p.key,
			clientSet: clientSet,
		}
		if err := p.reconcile(ctx, &req); err != nil {
			return fmt.Errorf("error reconciling '%s' phase: %v", p.name, err)
		}
	}
	return nil
}

func (r *ReplicationReconciler) configurePrimary(ctx context.Context, req *reconcileRequest) error {
	if req.mariadb.Status.CurrentPrimaryPodIndex != nil {
		return nil
	}
	client, err := req.clientSet.newPrimaryClient()
	if err != nil {
		return fmt.Errorf("error getting new primary client: %v", err)
	}

	if req.mariadb.Spec.Username != nil && req.mariadb.Spec.PasswordSecretKeyRef != nil {
		password, err := r.RefResolver.SecretKeyRef(ctx, *req.mariadb.Spec.PasswordSecretKeyRef, req.mariadb.Namespace)
		if err != nil {
			return fmt.Errorf("error getting password: %v", err)
		}
		userOpts := mariadbclient.CreateUserOpts{
			IdentifiedBy: password,
		}
		if err := client.CreateUser(ctx, *req.mariadb.Spec.Username, userOpts); err != nil {
			return fmt.Errorf("error creating user: %v", err)
		}

		grantOpts := mariadbclient.GrantOpts{
			Privileges:  []string{"ALL PRIVILEGES"},
			Database:    "*",
			Table:       "*",
			Username:    *req.mariadb.Spec.Username,
			GrantOption: false,
		}
		if err := client.Grant(ctx, grantOpts); err != nil {
			return fmt.Errorf("error creating grant: %v", err)
		}
	}

	if req.mariadb.Spec.Database != nil {
		databaseOpts := mariadbclient.DatabaseOpts{
			CharacterSet: "utf8",
			Collate:      "utf8_general_ci",
		}
		if err := client.CreateDatabase(ctx, *req.mariadb.Spec.Database, databaseOpts); err != nil {
			return fmt.Errorf("error creating database: %v", err)
		}
	}

	password, err := r.reconcileReplPasswordSecret(ctx, req.mariadb)
	if err != nil {
		return fmt.Errorf("error reconciling replication passsword: %v", err)
	}
	userOpts := mariadbclient.CreateUserOpts{
		IdentifiedBy: password,
	}
	if err := client.CreateUser(ctx, ReplUser, userOpts); err != nil {
		return fmt.Errorf("error creating replication user: %v", err)
	}
	grantOpts := mariadbclient.GrantOpts{
		Privileges:  []string{"REPLICATION REPLICA"},
		Database:    "*",
		Table:       "*",
		Username:    ReplUser,
		GrantOption: false,
	}
	if err := client.Grant(ctx, grantOpts); err != nil {
		return fmt.Errorf("error creating grant: %v", err)
	}

	config := primaryConfig{
		mariadb: req.mariadb,
		client:  client,
		ordinal: req.mariadb.Spec.Replication.PrimaryPodIndex,
	}
	if err := r.configurePrimaryVars(ctx, &config); err != nil {
		return fmt.Errorf("error configuring primary vars: %v", err)
	}
	return nil
}

func (r *ReplicationReconciler) configureReplicas(ctx context.Context, req *reconcileRequest) error {
	if req.mariadb.Status.CurrentPrimaryPodIndex != nil {
		return nil
	}
	var replSecret corev1.Secret
	if err := r.Get(ctx, replPasswordKey(req.mariadb), &replSecret); err != nil {
		return fmt.Errorf("error getting replication password Secret: %v", err)
	}
	gtid, err := req.mariadb.Spec.Replication.Gtid.MariaDBFormat()
	if err != nil {
		return fmt.Errorf("error getting GTID: %v", err)
	}
	for i := 0; i < int(req.mariadb.Spec.Replicas); i++ {
		if i == req.mariadb.Spec.Replication.PrimaryPodIndex {
			continue
		}
		client, err := req.clientSet.replicaClient(i)
		if err != nil {
			return fmt.Errorf("error getting client for replica '%d': %v", i, err)
		}

		config := replicaConfig{
			mariadb: req.mariadb,
			client:  client,
			changeMasterOpts: &mariadbclient.ChangeMasterOpts{
				Connection: ConnectionName,
				Host: statefulset.PodFQDN(
					req.mariadb.ObjectMeta,
					req.mariadb.Spec.Replication.PrimaryPodIndex,
				),
				User:     ReplUser,
				Password: string(replSecret.Data[PasswordSecretKey]),
				Gtid:     gtid,
			},
			ordinal: i,
		}
		if err := r.configureReplicaVars(ctx, &config); err != nil {
			return fmt.Errorf("error configuring replica vars in replica '%d': %v", err, i)
		}
	}
	return nil
}

func (r *ReplicationReconciler) reconcilePodDisruptionBudget(ctx context.Context, req *reconcileRequest) error {
	if req.mariadb.Spec.PodDisruptionBudget != nil {
		return nil
	}

	key := replresources.PodDisruptionBudgetKey(req.mariadb)
	var existingPDB policyv1.PodDisruptionBudget
	if err := r.Get(ctx, key, &existingPDB); err == nil {
		return nil
	}

	selectorLabels :=
		labels.NewLabelsBuilder().
			WithMariaDB(req.mariadb).
			Build()
	minAvailable := intstr.FromString("50%")
	opts := builder.PodDisruptionBudgetOpts{
		Key:            key,
		MinAvailable:   &minAvailable,
		SelectorLabels: selectorLabels,
	}
	pdb, err := r.Builder.BuildPodDisruptionBudget(&opts, req.mariadb)
	if err != nil {
		return fmt.Errorf("error building PodDisruptionBudget: %v", err)
	}

	if err := r.Create(ctx, pdb); err != nil {
		return fmt.Errorf("error creating PodDisruptionBudget: %v", err)
	}
	return nil
}

func (r *ReplicationReconciler) reconcilePrimaryService(ctx context.Context, req *reconcileRequest) error {
	serviceLabels :=
		labels.NewLabelsBuilder().
			WithMariaDB(req.mariadb).
			WithStatefulSetPod(req.mariadb, req.mariadb.Spec.Replication.PrimaryPodIndex).
			Build()
	opts := builder.ServiceOpts{
		Labels: serviceLabels,
	}
	if req.mariadb.Spec.Replication.PrimaryService != nil {
		opts.Type = req.mariadb.Spec.Replication.PrimaryService.Type
		opts.Annotations = req.mariadb.Spec.Replication.PrimaryService.Annotations
	}
	desiredSvc, err := r.Builder.BuildService(req.mariadb, req.key, opts)
	if err != nil {
		return fmt.Errorf("error building Service: %v", err)
	}

	var existingSvc corev1.Service
	if err := r.Get(ctx, req.key, &existingSvc); err != nil {
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

func (r *ReplicationReconciler) reconcilePrimaryConn(ctx context.Context, req *reconcileRequest) error {
	if req.mariadb.Spec.Connection == nil || req.mariadb.Spec.Username == nil || req.mariadb.Spec.PasswordSecretKeyRef == nil {
		return nil
	}
	var existingConn mariadbv1alpha1.Connection
	if err := r.Get(ctx, req.key, &existingConn); err == nil {
		return nil
	}

	connTpl := req.mariadb.Spec.Replication.PrimaryConnection
	if req.mariadb.Spec.Replication != nil {
		serviceName := replresources.PrimaryServiceKey(req.mariadb).Name
		connTpl.ServiceName = &serviceName
	}

	connOpts := builder.ConnectionOpts{
		Key: req.key,
		MariaDBRef: mariadbv1alpha1.MariaDBRef{
			LocalObjectReference: corev1.LocalObjectReference{
				Name: req.mariadb.Name,
			},
			WaitForIt: true,
		},
		Username:             *req.mariadb.Spec.Username,
		PasswordSecretKeyRef: *req.mariadb.Spec.PasswordSecretKeyRef,
		Database:             req.mariadb.Spec.Database,
		Template:             connTpl,
	}
	conn, err := r.Builder.BuildConnection(connOpts, req.mariadb)
	if err != nil {
		return fmt.Errorf("erro building primary Connection: %v", err)
	}

	if err := r.Create(ctx, conn); err != nil {
		return fmt.Errorf("error creating primary Connection: %v", err)
	}
	return nil
}

func (r *ReplicationReconciler) updateCurrentPrimaryPodIndex(ctx context.Context, req *reconcileRequest) error {
	if req.mariadb.Status.CurrentPrimaryPodIndex != nil {
		return nil
	}
	if err := r.patchStatus(ctx, req.mariadb, func(status *mariadbv1alpha1.MariaDBStatus) error {
		status.CurrentPrimaryPodIndex = &req.mariadb.Spec.Replication.PrimaryPodIndex
		return nil
	}); err != nil {
		return fmt.Errorf("error patching MariaDB status: %v", err)
	}
	return nil
}

func (r *ReplicationReconciler) reconcileReplPasswordSecret(ctx context.Context,
	mariadb *mariadbv1alpha1.MariaDB) (string, error) {
	password, err := password.Generate(16, 4, 0, false, false)
	if err != nil {
		return "", fmt.Errorf("error generating replication password: %v", err)
	}

	opts := builder.SecretOpts{
		Key: replPasswordKey(mariadb),
		Data: map[string][]byte{
			PasswordSecretKey: []byte(password),
		},
		Labels: labels.NewLabelsBuilder().WithMariaDB(mariadb).Build(),
	}
	secret, err := r.Builder.BuildSecret(opts, mariadb)
	if err != nil {
		return "", fmt.Errorf("error building replication password Secret: %v", err)
	}
	if err := r.Create(ctx, secret); err != nil {
		return "", fmt.Errorf("error creating replication password Secret: %v", err)
	}

	return password, nil
}

func (r *ReplicationReconciler) patchStatus(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	patcher func(*mariadbv1alpha1.MariaDBStatus) error) error {
	patch := client.MergeFrom(mariadb.DeepCopy())
	if err := patcher(&mariadb.Status); err != nil {
		return fmt.Errorf("errror calling MariaDB status patcher: %v", err)
	}

	if err := r.Client.Status().Patch(ctx, mariadb, patch); err != nil {
		return fmt.Errorf("error patching MariaDB status: %v", err)
	}
	return nil
}

func replPasswordKey(mariadb *mariadbv1alpha1.MariaDB) types.NamespacedName {
	return types.NamespacedName{
		Name:      fmt.Sprintf("repl-password-%s", mariadb.Name),
		Namespace: mariadb.Namespace,
	}
}
