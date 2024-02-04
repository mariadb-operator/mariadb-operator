package controller

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/pkg/builder"
	condition "github.com/mariadb-operator/mariadb-operator/pkg/condition"
	"github.com/mariadb-operator/mariadb-operator/pkg/health"
	"github.com/mariadb-operator/mariadb-operator/pkg/refresolver"
	clientsql "github.com/mariadb-operator/mariadb-operator/pkg/sql"
	"github.com/mariadb-operator/mariadb-operator/pkg/statefulset"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var (
	errConnHealthCheck = errors.New("error checking connection health")
)

// ConnectionReconciler reconciles a Connection object
type ConnectionReconciler struct {
	client.Client
	Scheme          *runtime.Scheme
	Builder         *builder.Builder
	RefResolver     *refresolver.RefResolver
	ConditionReady  *condition.Ready
	RequeueInterval time.Duration
}

//+kubebuilder:rbac:groups=mariadb.mmontes.io,resources=connections,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=mariadb.mmontes.io,resources=connections/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=mariadb.mmontes.io,resources=connections/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=secrets,verbs=list;watch;create;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *ConnectionReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var conn v1alpha1.Connection
	if err := r.Get(ctx, req.NamespacedName, &conn); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	mariadb, refErr := r.RefResolver.MariaDB(ctx, &conn.Spec.MariaDBRef, conn.Namespace)
	if refErr != nil {
		var mariaDbErr *multierror.Error
		mariaDbErr = multierror.Append(mariaDbErr, refErr)

		patchErr := r.patchStatus(ctx, &conn, r.ConditionReady.PatcherRefResolver(refErr, mariadb))
		mariaDbErr = multierror.Append(mariaDbErr, patchErr)

		return ctrl.Result{}, fmt.Errorf("error getting MariaDB: %v", mariaDbErr)
	}

	if conn.Spec.MariaDBRef.WaitForIt && !mariadb.IsReady() {
		if err := r.patchStatus(ctx, &conn, r.ConditionReady.PatcherFailed("MariaDB not ready")); err != nil {
			return ctrl.Result{}, fmt.Errorf("error patching Connection: %v", err)
		}
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	if err := r.init(ctx, &conn); err != nil {
		var initErr *multierror.Error
		initErr = multierror.Append(initErr, err)

		patchErr := r.patchStatus(
			ctx,
			&conn,
			r.ConditionReady.PatcherFailed(fmt.Sprintf("error initializing connection: %v", err)),
		)
		initErr = multierror.Append(initErr, patchErr)

		return ctrl.Result{}, fmt.Errorf("error initializing connection: %v", initErr)
	}

	healthy, err := health.IsMariaDBHealthy(ctx, r.Client, mariadb, health.EndpointPolicyAtLeastOne)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error checking MariaDB health: %v", err)
	}
	if !healthy {
		if err := r.patchStatus(ctx, &conn, r.ConditionReady.PatcherFailed("MariaDB not healthy")); err != nil {
			return ctrl.Result{}, fmt.Errorf("error patching Connection: %v", err)
		}
		return r.retryResult(&conn)
	}

	var secretErr *multierror.Error
	err = r.reconcileSecret(ctx, &conn, mariadb)
	if errors.Is(err, errConnHealthCheck) {
		return r.retryResult(&conn)
	}
	secretErr = multierror.Append(secretErr, err)

	patchErr := r.patchStatus(ctx, &conn, r.ConditionReady.PatcherHealthy(err))
	secretErr = multierror.Append(secretErr, patchErr)

	if err := secretErr.ErrorOrNil(); err != nil {
		return ctrl.Result{}, fmt.Errorf("error creating Secret: %v", err)
	}
	return r.healthResult(&conn)
}

func (r *ConnectionReconciler) init(ctx context.Context, conn *mariadbv1alpha1.Connection) error {
	if conn.IsInit() {
		return nil
	}
	patcher := func(c *mariadbv1alpha1.Connection) {
		c.Init()
	}
	if err := r.patch(ctx, conn, patcher); err != nil {
		return fmt.Errorf("error patching restore: %v", err)
	}
	return nil
}

func (r *ConnectionReconciler) reconcileSecret(ctx context.Context, conn *mariadbv1alpha1.Connection,
	mdb *mariadbv1alpha1.MariaDB) error {
	key := types.NamespacedName{
		Name:      conn.SecretName(),
		Namespace: conn.Namespace,
	}
	password, err := r.RefResolver.SecretKeyRef(ctx, conn.Spec.PasswordSecretKeyRef, conn.Namespace)
	if err != nil {
		return fmt.Errorf("error getting password for connection DSN: %v", err)
	}

	var host string
	if conn.Spec.ServiceName != nil {
		objMeta := metav1.ObjectMeta{
			Name:      *conn.Spec.ServiceName,
			Namespace: mdb.ObjectMeta.Namespace,
		}
		host = statefulset.ServiceFQDN(objMeta)
	} else {
		host = statefulset.ServiceFQDN(mdb.ObjectMeta)
	}
	mdbOpts := clientsql.Opts{
		Username: conn.Spec.Username,
		Password: password,
		Host:     host,
		Port:     mdb.Spec.Port,
		Params:   conn.Spec.Params,
	}
	if conn.Spec.Database != nil {
		mdbOpts.Database = *conn.Spec.Database
	}

	var existingSecret corev1.Secret
	if err := r.Get(ctx, key, &existingSecret); err == nil {
		if err := r.healthCheck(ctx, conn, mdbOpts); err != nil {
			log.FromContext(ctx).Info("Error checking connection health", "err", err)
			return errConnHealthCheck
		}
		return nil
	}

	dsn, err := clientsql.BuildDSN(mdbOpts)
	if err != nil {
		return fmt.Errorf("error building DSN: %v", err)
	}

	secretOpts := builder.SecretOpts{
		MariaDB: mdb,
		Key:     key,
		Data: map[string][]byte{
			conn.SecretKey(): []byte(dsn),
		},
		Labels:      conn.Spec.SecretTemplate.Labels,
		Annotations: conn.Spec.SecretTemplate.Annotations,
	}

	if formatString := conn.Spec.SecretTemplate.Format; formatString != nil {
		tmpl := template.Must(template.New("").Parse(*formatString))
		builder := &strings.Builder{}

		err := tmpl.Execute(builder, map[string]string{
			"Username": mdbOpts.Username,
			"Password": mdbOpts.Password,
			"Host":     mdbOpts.Host,
			"Port":     strconv.Itoa(int(mdbOpts.Port)),
			"Database": mdbOpts.Database,
			"Params": func() string {
				v := url.Values{}
				for key, value := range mdbOpts.Params {
					v.Add(key, value)
				}

				s := v.Encode()
				if s == "" {
					return s
				}
				return fmt.Sprintf("?%s", s)
			}(),
		})
		if err != nil {
			return fmt.Errorf("error parsing DSN template: %v", err)
		}
		secretOpts.Data[conn.SecretKey()] = []byte(builder.String())
	}
	if usernameKey := conn.Spec.SecretTemplate.UsernameKey; usernameKey != nil {
		secretOpts.Data[*usernameKey] = []byte(mdbOpts.Username)
	}
	if passwordKey := conn.Spec.SecretTemplate.PasswordKey; passwordKey != nil {
		secretOpts.Data[*passwordKey] = []byte(mdbOpts.Password)
	}
	if hostKey := conn.Spec.SecretTemplate.HostKey; hostKey != nil {
		secretOpts.Data[*hostKey] = []byte(mdbOpts.Host)
	}
	if portKey := conn.Spec.SecretTemplate.PortKey; portKey != nil {
		secretOpts.Data[*portKey] = []byte(strconv.Itoa(int(mdbOpts.Port)))
	}
	if databaseKey := conn.Spec.SecretTemplate.DatabaseKey; databaseKey != nil && mdbOpts.Database != "" {
		secretOpts.Data[*databaseKey] = []byte(mdbOpts.Database)
	}

	secret, err := r.Builder.BuildSecret(secretOpts, conn)
	if err != nil {
		return fmt.Errorf("error building Secret: %v", err)
	}

	if err := r.Create(ctx, secret); err != nil {
		return fmt.Errorf("error creating Secret: %v", err)
	}
	return nil
}

func (r *ConnectionReconciler) healthCheck(ctx context.Context, conn *mariadbv1alpha1.Connection, clientOpts clientsql.Opts) error {
	log.FromContext(ctx).V(1).Info("Checking connection health")
	db, err := clientsql.ConnectWithOpts(clientOpts)
	if err != nil {
		var connErr *multierror.Error
		connErr = multierror.Append(connErr, err)

		patchErr := r.patchStatus(
			ctx,
			conn,
			r.ConditionReady.PatcherHealthy(fmt.Errorf("failed to connect: %v", err)),
		)
		return multierror.Append(connErr, patchErr)
	}
	defer db.Close()

	if err := r.patchStatus(ctx, conn, r.ConditionReady.PatcherHealthy(nil)); err != nil {
		return fmt.Errorf("error patching connection status: %v", err)
	}
	return nil
}

func (r *ConnectionReconciler) retryResult(conn *mariadbv1alpha1.Connection) (ctrl.Result, error) {
	if conn.Spec.HealthCheck != nil && conn.Spec.HealthCheck.RetryInterval != nil {
		return ctrl.Result{RequeueAfter: (*conn.Spec.HealthCheck.RetryInterval).Duration}, nil
	}
	return ctrl.Result{RequeueAfter: r.RequeueInterval}, nil
}

func (r *ConnectionReconciler) healthResult(conn *mariadbv1alpha1.Connection) (ctrl.Result, error) {
	if conn.Spec.HealthCheck != nil && conn.Spec.HealthCheck.Interval != nil {
		return ctrl.Result{RequeueAfter: (*conn.Spec.HealthCheck.Interval).Duration}, nil
	}
	return ctrl.Result{}, nil
}

func (r *ConnectionReconciler) patchStatus(ctx context.Context, conn *mariadbv1alpha1.Connection,
	patcher condition.Patcher) error {
	patch := client.MergeFrom(conn.DeepCopy())
	patcher(&conn.Status)

	if err := r.Client.Status().Patch(ctx, conn, patch); err != nil {
		return fmt.Errorf("error patching connection status: %v", err)
	}
	return nil
}

func (r *ConnectionReconciler) patch(ctx context.Context, conn *mariadbv1alpha1.Connection,
	patcher func(*mariadbv1alpha1.Connection)) error {
	patch := client.MergeFrom(conn.DeepCopy())
	patcher(conn)

	if err := r.Client.Patch(ctx, conn, patch); err != nil {
		return fmt.Errorf("error patching connection: %v", err)
	}
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ConnectionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&mariadbv1alpha1.Connection{}).
		Owns(&corev1.Secret{}).
		Complete(r)
}
