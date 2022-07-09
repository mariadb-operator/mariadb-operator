package reconcilers

import (
	"context"
	"fmt"
	"time"

	databasev1alpha1 "github.com/mmontes11/mariadb-operator/api/v1alpha1"
	"github.com/mmontes11/mariadb-operator/pkg/builders"
	"github.com/mmontes11/mariadb-operator/pkg/mariadb"
	mariadbclient "github.com/mmontes11/mariadb-operator/pkg/mariadb"
	"github.com/mmontes11/mariadb-operator/pkg/refresolver"
	"github.com/sethvargo/go-password/password"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

var (
	exporterPrivileges = []string{
		"SELECT",
		"PROCESS",
		// TODO: check MariaDB version and use 'REPLICATION CLIENT' instead
		// see: https://mariadb.com/kb/en/grant/#binlog-monitor
		"BINLOG MONITOR",
		"SLAVE MONITOR",
	}
	passwordSecretKey = "password"
	dsnSecretKey      = "dsn"
)

type ExporterReconciler struct {
	client.Client
	scheme      *runtime.Scheme
	refResolver *refresolver.RefResolver
}

func NewExporterReonciler(client client.Client, scheme *runtime.Scheme, refResolver *refresolver.RefResolver) *ExporterReconciler {
	return &ExporterReconciler{
		Client:      client,
		scheme:      scheme,
		refResolver: refResolver,
	}
}

func (r *ExporterReconciler) CreateExporter(ctx context.Context, mariadb *databasev1alpha1.MariaDB,
	monitor *databasev1alpha1.MonitorMariaDB, mdbClient *mariadb.Client) error {
	user, err := r.createExporterCredentials(ctx, mariadb, monitor, mdbClient)
	if err != nil {
		return fmt.Errorf("error creating exporter credentials: %v", err)
	}
	if err := r.createExporterDeployment(ctx, mariadb, monitor, user); err != nil {
		return fmt.Errorf("error creating exporter deployment: %v", err)
	}
	return nil
}

func (r *ExporterReconciler) createExporterCredentials(ctx context.Context, mariadb *databasev1alpha1.MariaDB,
	monitor *databasev1alpha1.MonitorMariaDB, mdbClient *mariadb.Client) (*databasev1alpha1.UserMariaDB, error) {
	key := exporterKey(mariadb)
	exists, err := mdbClient.UserExists(ctx, key.Name)
	if err != nil {
		return nil, fmt.Errorf("error checking if user exists: %v", err)
	}
	hasPrivileges, err := mdbClient.UserHasPrivileges(ctx, key.Name, exporterPrivileges)
	if err != nil {
		return nil, fmt.Errorf("error checking user privileges: %v", err)
	}
	if exists && hasPrivileges {
		var existingUser databasev1alpha1.UserMariaDB
		if err := r.Get(ctx, key, &existingUser); err != nil {
			return nil, fmt.Errorf("error getting UserMariaDB on API server: %v", err)
		}
		return &existingUser, nil
	}

	if err := r.createUser(ctx, mariadb, monitor); err != nil {
		return nil, fmt.Errorf("error creating UserMariaDB: %v", err)
	}
	var user databasev1alpha1.UserMariaDB
	err = wait.PollImmediateWithContext(ctx, 1*time.Second, 30*time.Second, func(ctx context.Context) (bool, error) {
		if r.Get(ctx, key, &user) != nil {
			return false, nil
		}
		return user.IsReady(), nil
	})
	if err != nil {
		return nil, fmt.Errorf("error creating UserMariaDB: %v", err)
	}

	if err := r.createGrant(ctx, mariadb, monitor, &user); err != nil {
		return nil, fmt.Errorf("error creating GrantMariaDB: %v", err)
	}
	var grant databasev1alpha1.GrantMariaDB
	err = wait.PollImmediateWithContext(ctx, 1*time.Second, 30*time.Second, func(ctx context.Context) (bool, error) {
		if r.Get(ctx, key, &grant) != nil {
			return false, nil
		}
		return grant.IsReady(), nil
	})
	if err != nil {
		return nil, fmt.Errorf("error creating GrantMariaDB: %v", err)
	}

	return &user, nil
}

func (r *ExporterReconciler) createExporterDeployment(ctx context.Context, mariadb *databasev1alpha1.MariaDB,
	monitor *databasev1alpha1.MonitorMariaDB, user *databasev1alpha1.UserMariaDB) error {
	key := exporterKey(mariadb)
	var existingDeploy v1.Deployment
	if err := r.Get(ctx, key, &existingDeploy); err == nil {
		return nil
	}

	dsnSecretKeySelector, err := r.createDsnSecret(ctx, mariadb, monitor, user)
	if err != nil {
		return fmt.Errorf("error creating DSN Secret: %v", err)
	}
	deploy, err := builders.BuildExporterDeployment(mariadb, monitor, dsnSecretKeySelector)
	if err != nil {
		return fmt.Errorf("error building exporter Deployment: %v", err)
	}
	if err := controllerutil.SetControllerReference(monitor, deploy, r.scheme); err != nil {
		return fmt.Errorf("error setting controller reference to exporter Deployment: %v", err)
	}

	if err := r.Create(ctx, deploy); err != nil {
		return fmt.Errorf("error creating exporter Deployment in API server: %v", err)
	}
	return nil
}

func (r *ExporterReconciler) createUser(ctx context.Context, mariadb *databasev1alpha1.MariaDB,
	monitor *databasev1alpha1.MonitorMariaDB) error {
	key := exporterKey(mariadb).Name
	secretKeySelector, err := r.createPasswordSecret(ctx, mariadb, monitor)
	if err != nil {
		return fmt.Errorf("error creating user password: %v", err)
	}

	opts := builders.UserOpts{
		Name:                 key,
		PasswordSecretKeyRef: *secretKeySelector,
		MaxUserConnections:   3,
	}
	user := builders.BuildUser(mariadb, opts)
	if err := controllerutil.SetControllerReference(monitor, user, r.scheme); err != nil {
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
	if err := controllerutil.SetControllerReference(monitor, grant, r.scheme); err != nil {
		return fmt.Errorf("error setting controller reference to GrantMariaDB: %v", err)
	}

	return r.Create(ctx, grant)
}

func (r *ExporterReconciler) createPasswordSecret(ctx context.Context, mariadb *databasev1alpha1.MariaDB,
	monitor *databasev1alpha1.MonitorMariaDB) (*corev1.SecretKeySelector, error) {
	password, err := password.Generate(16, 4, 0, false, false)
	if err != nil {
		return nil, fmt.Errorf("error generating passowrd: %v", err)
	}

	opts := builders.SecretOpts{
		Name: passwordKey(mariadb).Name,
		Data: map[string][]byte{
			passwordSecretKey: []byte(password),
		},
	}
	secret := builders.BuildSecret(mariadb, opts)
	if err := controllerutil.SetControllerReference(monitor, secret, r.scheme); err != nil {
		return nil, fmt.Errorf("error setting controller reference to password Secret: %v", err)
	}
	if err := r.Create(ctx, secret); err != nil {
		return nil, fmt.Errorf("error creating password Secret on API server: %v", err)
	}

	return &corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{
			Name: secret.Name,
		},
		Key: passwordSecretKey,
	}, nil
}

func (r *ExporterReconciler) createDsnSecret(ctx context.Context, mariadb *databasev1alpha1.MariaDB,
	monitor *databasev1alpha1.MonitorMariaDB, user *databasev1alpha1.UserMariaDB) (*corev1.SecretKeySelector, error) {
	password, err := r.refResolver.ReadSecretKeyRef(ctx, user.Spec.PasswordSecretKeyRef, mariadb.Namespace)
	if err != nil {
		return nil, fmt.Errorf("error getting password: %v", err)
	}
	mdbOpts := mariadbclient.Opts{
		Username: user.Name,
		Password: password,
		Host:     mariadb.Name,
		Port:     mariadb.Spec.Port,
	}
	dsn, err := mariadbclient.BuildDSN(mdbOpts)
	if err != nil {
		return nil, fmt.Errorf("error building DSN: %v", err)
	}

	secretOpts := builders.SecretOpts{
		Name: dsnKey(mariadb).Name,
		Data: map[string][]byte{
			dsnSecretKey: []byte(dsn),
		},
	}
	secret := builders.BuildSecret(mariadb, secretOpts)
	if err := controllerutil.SetControllerReference(monitor, secret, r.scheme); err != nil {
		return nil, fmt.Errorf("error setting controller reference to DSN Secret: %v", err)
	}
	if err := r.Create(ctx, secret); err != nil {
		return nil, fmt.Errorf("error creating DSN Secret on API server: %v", err)
	}
	return &corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{
			Name: secret.Name,
		},
		Key: dsnSecretKey,
	}, nil
}

func exporterKey(mariadb *databasev1alpha1.MariaDB) types.NamespacedName {
	return types.NamespacedName{
		Name:      fmt.Sprintf("%s-exporter", mariadb.Name),
		Namespace: mariadb.Namespace,
	}
}

func passwordKey(mariadb *databasev1alpha1.MariaDB) types.NamespacedName {
	return types.NamespacedName{
		Name:      fmt.Sprintf("%s-exporter-password", mariadb.Name),
		Namespace: mariadb.Namespace,
	}
}

func dsnKey(mariadb *databasev1alpha1.MariaDB) types.NamespacedName {
	return types.NamespacedName{
		Name:      fmt.Sprintf("%s-exporter-dsn", mariadb.Name),
		Namespace: mariadb.Namespace,
	}
}
