package reconcilers

import (
	"context"
	"fmt"
	"time"

	databasev1alpha1 "github.com/mmontes11/mariadb-operator/api/v1alpha1"
	"github.com/mmontes11/mariadb-operator/pkg/builders"
	"github.com/mmontes11/mariadb-operator/pkg/mariadb"
	"github.com/sethvargo/go-password/password"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

var (
	exporterPrivileges = []string{
		"PROCESS",
		// TODO: check MariaDB version and use 'REPLICATION CLIENT' instead
		// see: https://mariadb.com/kb/en/grant/#binlog-monitor
		"BINLOG MONITOR",
		"SELECT",
	}
)

type ExporterReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func NewExporterReonciler(client client.Client, scheme *runtime.Scheme) *ExporterReconciler {
	return &ExporterReconciler{
		Client: client,
		Scheme: scheme,
	}
}

func (r *ExporterReconciler) CreateExporter(ctx context.Context, mariadb *databasev1alpha1.MariaDB,
	monitor *databasev1alpha1.MonitorMariaDB, mdbClient *mariadb.Client) error {

	if err := r.createExporterCredentials(ctx, mariadb, monitor, mdbClient); err != nil {
		return fmt.Errorf("error creating exporter credentials: %v", err)
	}

	return nil
}

func (r *ExporterReconciler) createExporterCredentials(ctx context.Context, mariadb *databasev1alpha1.MariaDB,
	monitor *databasev1alpha1.MonitorMariaDB, mdbClient *mariadb.Client) error {
	key := exporterKey(mariadb)
	exists, err := mdbClient.UserExists(ctx, key.Name)
	if err != nil {
		return fmt.Errorf("error checking if user exists: %v", err)
	}
	hasPrivileges, err := mdbClient.UserHasPrivileges(ctx, key.Name, exporterPrivileges)
	if err != nil {
		return fmt.Errorf("error checking user privileges: %v", err)
	}
	if exists && hasPrivileges {
		return nil
	}

	if err := r.createUser(ctx, mariadb, monitor); err != nil {
		return fmt.Errorf("error creating UserMariaDB: %v", err)
	}
	var user databasev1alpha1.UserMariaDB
	err = wait.PollImmediateWithContext(ctx, 1*time.Second, 30*time.Second, func(ctx context.Context) (bool, error) {
		if r.Get(ctx, key, &user) != nil {
			return false, nil
		}
		return user.IsReady(), nil
	})
	if err != nil {
		return fmt.Errorf("error creating UserMariaDB: %v", err)
	}

	if err := r.createGrant(ctx, mariadb, monitor, &user); err != nil {
		return fmt.Errorf("error creating GrantMariaDB: %v", err)
	}
	var grant databasev1alpha1.GrantMariaDB
	err = wait.PollImmediateWithContext(ctx, 1*time.Second, 30*time.Second, func(ctx context.Context) (bool, error) {
		if r.Get(ctx, key, &grant) != nil {
			return false, nil
		}
		return grant.IsReady(), nil
	})
	if err != nil {
		return fmt.Errorf("error creating GrantMariaDB: %v", err)
	}

	return nil
}

func (r *ExporterReconciler) createUser(ctx context.Context, mariadb *databasev1alpha1.MariaDB,
	monitor *databasev1alpha1.MonitorMariaDB) error {
	key := exporterKey(mariadb).Name
	secretKeySelector, err := r.createPassword(ctx, mariadb, monitor)
	if err != nil {
		return fmt.Errorf("error creating user password: %v", err)
	}

	opts := builders.UserOpts{
		Name:                 key,
		PasswordSecretKeyRef: *secretKeySelector,
		MaxUserConnections:   3,
	}
	user := builders.BuildUser(mariadb, opts)
	if err := controllerutil.SetControllerReference(monitor, user, r.Scheme); err != nil {
		return fmt.Errorf("error setting controller reference to UserMariaDB: %v", err)
	}

	return r.Create(ctx, user)
}

func (r *ExporterReconciler) createGrant(ctx context.Context, mariadb *databasev1alpha1.MariaDB,
	monitor *databasev1alpha1.MonitorMariaDB, user *databasev1alpha1.UserMariaDB) error {
	opts := builders.GrantOpts{
		Name:        exporterKey(mariadb).Name,
		Privileges:  exporterPrivileges,
		Database:    "*",
		Table:       "*",
		Username:    user.Name,
		GrantOption: false,
	}
	grant := builders.BuildGrant(mariadb, opts)
	if err := controllerutil.SetControllerReference(monitor, grant, r.Scheme); err != nil {
		return fmt.Errorf("error setting controller reference to GrantMariaDB: %v", err)
	}

	return r.Create(ctx, grant)
}

func (r *ExporterReconciler) createPassword(ctx context.Context, mariadb *databasev1alpha1.MariaDB,
	monitor *databasev1alpha1.MonitorMariaDB) (*corev1.SecretKeySelector, error) {
	password, err := password.Generate(64, 10, 10, false, false)
	if err != nil {
		return nil, fmt.Errorf("error generating passowrd: %v", err)
	}

	secretKey := "password"
	opts := builders.SecretOpts{
		Name: exporterKey(mariadb).Name,
		Data: map[string][]byte{
			secretKey: []byte(password),
		},
	}
	secret := builders.BuildSecret(mariadb, opts)
	if err := controllerutil.SetControllerReference(monitor, secret, r.Scheme); err != nil {
		return nil, fmt.Errorf("error setting controller reference to Secret: %v", err)
	}
	if err := r.Client.Create(ctx, secret); err != nil {
		return nil, fmt.Errorf("error creating Secret on API server: %v", err)
	}

	return &corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{
			Name: exporterKey(mariadb).Name,
		},
		Key: secretKey,
	}, nil
}

func exporterKey(mariadb *databasev1alpha1.MariaDB) types.NamespacedName {
	return types.NamespacedName{
		Name:      fmt.Sprintf("%s-exporter", mariadb.Name),
		Namespace: mariadb.Namespace,
	}
}
