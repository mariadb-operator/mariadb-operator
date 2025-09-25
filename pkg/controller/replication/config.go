package replication

import (
	"context"
	"fmt"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v25/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/builder"
	builderpki "github.com/mariadb-operator/mariadb-operator/v25/pkg/builder/pki"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/controller/auth"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/controller/secret"
	env "github.com/mariadb-operator/mariadb-operator/v25/pkg/environment"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/refresolver"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/sql"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/statefulset"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/version"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var (
	replUser     = "repl"
	replUserHost = "%"
)

type ReplicationConfig struct {
	client.Client
	builder          *builder.Builder
	refResolver      *refresolver.RefResolver
	secretReconciler *secret.SecretReconciler
	authReconciler   *auth.AuthReconciler
	env              *env.OperatorEnv
}

func NewReplicationConfig(client client.Client, builder *builder.Builder, secretReconciler *secret.SecretReconciler,
	authReconciler *auth.AuthReconciler, env *env.OperatorEnv) *ReplicationConfig {
	return &ReplicationConfig{
		Client:           client,
		builder:          builder,
		refResolver:      refresolver.New(client),
		secretReconciler: secretReconciler,
		authReconciler:   authReconciler,
		env:              env,
	}
}

// ConfigurePrimary will configure a primary replica given the pod's index
func (r *ReplicationConfig) ConfigurePrimary(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB, client *sql.Client,
	podIndex int) (ctrl.Result, error) {
	if err := client.StopAllSlaves(ctx); err != nil {
		return ctrl.Result{}, fmt.Errorf("error stopping slaves: %v", err)
	}
	if err := client.ResetAllSlaves(ctx); err != nil {
		return ctrl.Result{}, fmt.Errorf("error resetting slave: %v", err)
	}
	if err := client.ResetSlavePos(ctx); err != nil {
		return ctrl.Result{}, fmt.Errorf("error resetting slave position: %v", err)
	}
	if err := client.DisableReadOnly(ctx); err != nil {
		return ctrl.Result{}, fmt.Errorf("error disabling read_only: %v", err)
	}
	// @TODO: This should probably be a functionality of the AuthReconciler. If it does not exist and Generate is true, we can create it
	if err := r.reconcileReplUserPassword(ctx, mariadb); err != nil {
		return ctrl.Result{}, fmt.Errorf("error while creating password for replication user: %v", err)
	}

	if result, err := r.reconcileSQL(ctx, mariadb, client); !result.IsZero() || err != nil {
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("error reconciling primary SQL: %v", err)
		}

		return result, err
	}
	if result, err := r.configurePrimaryVars(ctx, mariadb, client, podIndex); !result.IsZero() || err != nil {
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("error configuring replication variables: %v", err)
		}

		return result, err
	}
	return ctrl.Result{}, nil
}

func (r *ReplicationConfig) ConfigureReplica(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB, client *sql.Client,
	replicaPodIndex, primaryPodIndex int, resetSlavePos bool) error {
	if err := client.ResetMaster(ctx); err != nil {
		return fmt.Errorf("error resetting master: %v", err)
	}
	if err := client.StopAllSlaves(ctx); err != nil {
		return fmt.Errorf("error stopping slaves: %v", err)
	}
	if resetSlavePos {
		if err := client.ResetSlavePos(ctx); err != nil {
			return fmt.Errorf("error resetting slave position: %v", err)
		}
	}
	if err := client.EnableReadOnly(ctx); err != nil {
		return fmt.Errorf("error enabling read_only: %v", err)
	}
	if err := r.configureReplicaVars(ctx, mariadb, client, replicaPodIndex); err != nil {
		return fmt.Errorf("error configuring replication variables: %v", err)
	}
	if err := r.changeMaster(ctx, mariadb, client, primaryPodIndex); err != nil {
		return fmt.Errorf("error changing master: %v", err)
	}
	if err := client.StartSlave(ctx); err != nil {
		return fmt.Errorf("error starting slave: %v", err)
	}
	return nil
}

func (r *ReplicationConfig) configurePrimaryVars(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB, client *sql.Client,
	primaryPodIndex int) (ctrl.Result, error) {
	log.FromContext(ctx).Info("Configuring Primary Vars", "primaryPodIndex", primaryPodIndex)
	kv := map[string]string{
		"sync_binlog":                  fmt.Sprintf("%d", ptr.Deref(mariadb.Replication().SyncBinlog, 1)),
		"rpl_semi_sync_master_enabled": "ON",
		"rpl_semi_sync_master_timeout": func() string {
			return fmt.Sprint(mariadb.Replication().Replica.ConnectionTimeout.Milliseconds())
		}(),
		"rpl_semi_sync_slave_enabled": "OFF",
		"server_id":                   serverId(primaryPodIndex),
	}
	if mariadb.Replication().Replica.WaitPoint != nil {
		waitPoint, err := mariadb.Replication().Replica.WaitPoint.MariaDBFormat()
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("error getting wait point: %v", err)
		}
		kv["rpl_semi_sync_master_wait_point"] = waitPoint
	}
	if err := client.SetSystemVariables(ctx, kv); err != nil {
		return ctrl.Result{}, fmt.Errorf("error setting replication vars: %v", err)
	}
	return ctrl.Result{}, nil
}

func (r *ReplicationConfig) configureReplicaVars(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	client *sql.Client, ordinal int) error {
	log.FromContext(ctx).Info("Configuring Replica Vars", "ordinal", ordinal)
	kv := map[string]string{
		"sync_binlog":                  fmt.Sprintf("%d", ptr.Deref(mariadb.Replication().SyncBinlog, 1)),
		"rpl_semi_sync_master_enabled": "OFF",
		"rpl_semi_sync_slave_enabled":  "ON",
		"server_id":                    serverId(ordinal),
	}
	if err := client.SetSystemVariables(ctx, kv); err != nil {
		return fmt.Errorf("error setting replication vars: %v", err)
	}
	return nil
}

func (r *ReplicationConfig) changeMaster(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB, client *sql.Client,
	primaryPodIndex int) error {
	replPasswordRef := mariadb.Spec.Replication.Replica.ReplPasswordSecretKeyRef
	password, err := r.refResolver.SecretKeyRef(ctx, replPasswordRef.SecretKeySelector, mariadb.Namespace)
	if err != nil {
		return fmt.Errorf("error getting replication password: %v", err)
	}

	gtid := mariadbv1alpha1.GtidCurrentPos
	if mariadb.Replication().Replica.Gtid != nil {
		gtid = *mariadb.Replication().Replica.Gtid
	}
	gtidString, err := gtid.MariaDBFormat()
	if err != nil {
		return fmt.Errorf("error getting GTID: %v", err)
	}

	changeMasterHostOpt, err := r.getChangeMasterHost(ctx, mariadb, primaryPodIndex)
	if err != nil {
		return fmt.Errorf("error getting host option: %v", err)
	}

	changeMasterOpts := []sql.ChangeMasterOpt{
		changeMasterHostOpt,
		sql.WithChangeMasterPort(mariadb.Spec.Port),
		sql.WithChangeMasterCredentials(replUser, password),
		sql.WithChangeMasterGtid(gtidString),
		sql.WithChangeMasterRetries(*mariadb.Replication().Replica.ConnectionRetries),
	}
	if mariadb.IsTLSEnabled() {
		changeMasterOpts = append(changeMasterOpts, sql.WithChangeMasterSSL(
			builderpki.ClientCertPath,
			builderpki.ClientKeyPath,
			builderpki.CACertPath,
		))
	}
	if err := client.ChangeMaster(ctx, changeMasterOpts...); err != nil {
		return fmt.Errorf("error changing master: %v", err)
	}
	return nil
}

func (r *ReplicationConfig) getChangeMasterHost(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	primaryPodIndex int) (sql.ChangeMasterOpt, error) {
	logger := log.FromContext(ctx).
		WithName("replication-config").
		WithValues("image", mariadb.Spec.Image).
		V(1)
	vOpts := []version.Option{
		version.WithLogger(logger),
	}
	if r.env != nil && r.env.MariadbDefaultVersion != "" {
		vOpts = append(vOpts, version.WithDefaultVersion(r.env.MariadbDefaultVersion))
	}
	v, err := version.NewVersion(mariadb.Spec.Image, vOpts...)
	if err != nil {
		return nil, fmt.Errorf("error creating version: %v", err)
	}

	isCompatibleVersion, err := v.GreaterThanOrEqual("10.6")
	if err != nil {
		return nil, fmt.Errorf("error comparing version: %v", err)
	}

	if isCompatibleVersion {
		return sql.WithChangeMasterHost(
			statefulset.PodFQDNWithService(
				mariadb.ObjectMeta,
				primaryPodIndex,
				mariadb.InternalServiceKey().Name,
			),
		), nil
	}
	return sql.WithChangeMasterHost(
		// MariaDB 10.5 has a limitation of 60 characters in this host.
		statefulset.PodShortFQDNWithService(
			mariadb.ObjectMeta,
			primaryPodIndex,
			mariadb.InternalServiceKey().Name,
		),
	), nil
}

// Creates a `User` and `Grant` resources with minimum required permissions for replication.
func (r *ReplicationConfig) reconcileSQL(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	client *sql.Client) (ctrl.Result, error) {
	replUserKey := mariadb.MariadbReplUserKey()
	replGrantKey := mariadb.MariadbReplGrantKey()

	userOpts := builder.UserOpts{
		MariaDBRef: mariadbv1alpha1.MariaDBRef{
			ObjectReference: mariadbv1alpha1.ObjectReference{
				Name:      mariadb.Name,
				Namespace: mariadb.Namespace,
			},
		},
		Metadata:             mariadb.Spec.InheritMetadata,
		MaxUserConnections:   20,
		Name:                 replUser,
		Host:                 replUserHost,
		PasswordSecretKeyRef: &mariadb.Spec.Replication.Replica.ReplPasswordSecretKeyRef.SecretKeySelector,
		CleanupPolicy:        ptr.To(mariadbv1alpha1.CleanupPolicySkip),
	}

	grantOpts := auth.GrantOpts{
		Key: replGrantKey,
		GrantOpts: builder.GrantOpts{
			MariaDBRef: mariadbv1alpha1.MariaDBRef{
				ObjectReference: mariadbv1alpha1.ObjectReference{
					Name:      mariadb.Name,
					Namespace: mariadb.Namespace,
				},
			},
			Metadata:      mariadb.Spec.InheritMetadata,
			Privileges:    []string{"REPLICATION REPLICA"},
			Database:      "*",
			Table:         "*",
			Username:      replUser,
			Host:          replUserHost,
			CleanupPolicy: ptr.To(mariadbv1alpha1.CleanupPolicySkip),
		},
	}

	if result, err := r.authReconciler.ReconcileUserGrant(ctx, replUserKey, mariadb, userOpts, grantOpts); !result.IsZero() || err != nil {
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("error reconciling %s user auth: %v", replUser, err)
		}
		return result, err
	}

	if result, err := r.authReconciler.WaitForGrant(ctx, replGrantKey); !result.IsZero() || err != nil {
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("error waiting for grant: %v", err)
		}
		return result, err
	}

	return ctrl.Result{}, nil
}

// reconcileReplUserPassword will create a new secret with repl user password if it does not already exists
func (r *ReplicationConfig) reconcileReplUserPassword(ctx context.Context, mdb *mariadbv1alpha1.MariaDB) error {
	secretKeyRef := mdb.Spec.Replication.Replica.ReplPasswordSecretKeyRef
	req := secret.PasswordRequest{
		Metadata: mdb.Spec.InheritMetadata,
		Owner:    mdb,
		Key: types.NamespacedName{
			Name:      secretKeyRef.Name,
			Namespace: mdb.Namespace,
		},
		SecretKey: secretKeyRef.Key,
		Generate:  secretKeyRef.Generate,
	}
	_, err := r.secretReconciler.ReconcilePassword(ctx, req)
	return err
}

func serverId(index int) string {
	return fmt.Sprint(10 + index)
}
