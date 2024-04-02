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
	corev1 "k8s.io/api/core/v1"
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

//+kubebuilder:rbac:groups=k8s.mariadb.com,resources=connections,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=k8s.mariadb.com,resources=connections/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=k8s.mariadb.com,resources=connections/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=secrets,verbs=list;watch;create;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *ConnectionReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var conn v1alpha1.Connection
	if err := r.Get(ctx, req.NamespacedName, &conn); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	connRefs, err := r.getRefs(ctx, &conn)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error getting references: %v", err)
	}

	if result, err := r.waitForRefs(ctx, &conn, connRefs); !result.IsZero() || err != nil {
		return result, err
	}

	if err := r.setDefaults(ctx, &conn, connRefs); err != nil {
		return ctrl.Result{}, fmt.Errorf("error setting defaults: %v", err)
	}

	if result, err := r.checkHealth(ctx, &conn, connRefs); !result.IsZero() || err != nil {
		return result, err
	}

	var secretErr *multierror.Error
	err = r.reconcileSecret(ctx, &conn, connRefs)
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

func (r *ConnectionReconciler) getRefs(ctx context.Context, conn *mariadbv1alpha1.Connection) (*mariadbv1alpha1.ConnectionRefs, error) {
	if conn.Spec.MariaDBRef != nil {
		mdb, refErr := r.RefResolver.MariaDB(ctx, conn.Spec.MariaDBRef, conn.Namespace)
		if refErr != nil {
			var mariaDbErr *multierror.Error
			mariaDbErr = multierror.Append(mariaDbErr, refErr)

			patchErr := r.patchStatus(ctx, conn, r.ConditionReady.PatcherRefResolver(refErr, mdb))
			mariaDbErr = multierror.Append(mariaDbErr, patchErr)

			return nil, fmt.Errorf("error getting MariaDB: %v", mariaDbErr)
		}
		return &mariadbv1alpha1.ConnectionRefs{
			MariaDB: mdb,
		}, nil
	}
	if conn.Spec.MaxScaleRef != nil {
		mxs, refErr := r.RefResolver.MaxScale(ctx, conn.Spec.MaxScaleRef, conn.Namespace)
		if refErr != nil {
			var mariaDbErr *multierror.Error
			mariaDbErr = multierror.Append(mariaDbErr, refErr)

			patchErr := r.patchStatus(ctx, conn, r.ConditionReady.PatcherRefResolver(refErr, mxs))
			mariaDbErr = multierror.Append(mariaDbErr, patchErr)

			return nil, fmt.Errorf("error getting MaxScale: %v", mariaDbErr)
		}
		return &mariadbv1alpha1.ConnectionRefs{
			MaxScale: mxs,
		}, nil
	}
	return nil, errors.New("no references found")
}

func (r *ConnectionReconciler) waitForRefs(ctx context.Context, conn *mariadbv1alpha1.Connection,
	refs *mariadbv1alpha1.ConnectionRefs) (ctrl.Result, error) {
	if conn.Spec.MariaDBRef != nil && refs.MariaDB != nil {
		if conn.Spec.MariaDBRef.WaitForIt && !refs.MariaDB.IsReady() {
			if err := r.patchStatus(ctx, conn, r.ConditionReady.PatcherFailed("MariaDB not ready")); err != nil {
				return ctrl.Result{}, fmt.Errorf("error patching Connection: %v", err)
			}
			return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
		}
	}
	if conn.Spec.MaxScaleRef != nil && refs.MaxScale != nil {
		if !refs.MaxScale.IsReady() {
			if err := r.patchStatus(ctx, conn, r.ConditionReady.PatcherFailed("MaxScale not ready")); err != nil {
				return ctrl.Result{}, fmt.Errorf("error patching Connection: %v", err)
			}
			return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
		}
	}
	return ctrl.Result{}, nil
}

func (r *ConnectionReconciler) setDefaults(ctx context.Context, conn *mariadbv1alpha1.Connection,
	refs *mariadbv1alpha1.ConnectionRefs) error {
	return r.patch(ctx, conn, func(conn *mariadbv1alpha1.Connection) error {
		return conn.SetDefaults(refs)
	})
}

func (r *ConnectionReconciler) checkHealth(ctx context.Context, conn *mariadbv1alpha1.Connection,
	refs *mariadbv1alpha1.ConnectionRefs) (ctrl.Result, error) {
	if refs.MariaDB != nil {
		healthy, err := health.IsStatefulSetHealthy(
			ctx,
			r.Client,
			client.ObjectKeyFromObject(refs.MariaDB),
			health.WithDesiredReplicas(refs.MariaDB.Spec.Replicas),
			health.WithPort(conn.Spec.Port),
			health.WithEndpointPolicy(health.EndpointPolicyAtLeastOne),
		)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("error checking MariaDB health: %v", err)
		}
		if !healthy {
			if err := r.patchStatus(ctx, conn, r.ConditionReady.PatcherFailed("MariaDB not healthy")); err != nil {
				return ctrl.Result{}, fmt.Errorf("error patching Connection: %v", err)
			}
			return r.retryResult(conn)
		}
	}
	if refs.MaxScale != nil {
		healthy, err := health.IsStatefulSetHealthy(
			ctx,
			r.Client,
			client.ObjectKeyFromObject(refs.MaxScale),
			health.WithDesiredReplicas(refs.MaxScale.Spec.Replicas),
			health.WithPort(conn.Spec.Port),
			health.WithEndpointPolicy(health.EndpointPolicyAtLeastOne),
		)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("error checking MaxScale health: %v", err)
		}
		if !healthy {
			if err := r.patchStatus(ctx, conn, r.ConditionReady.PatcherFailed("MaxScale not healthy")); err != nil {
				return ctrl.Result{}, fmt.Errorf("error patching Connection: %v", err)
			}
			return r.retryResult(conn)
		}
	}
	return ctrl.Result{}, nil
}

func (r *ConnectionReconciler) reconcileSecret(ctx context.Context, conn *mariadbv1alpha1.Connection,
	refs *mariadbv1alpha1.ConnectionRefs) error {
	key := types.NamespacedName{
		Name:      conn.SecretName(),
		Namespace: conn.Namespace,
	}
	password, err := r.RefResolver.SecretKeyRef(ctx, conn.Spec.PasswordSecretKeyRef, conn.Namespace)
	if err != nil {
		return fmt.Errorf("error getting password for connection DSN: %v", err)
	}

	mdbOpts := clientsql.Opts{
		Username: conn.Spec.Username,
		Password: password,
		Host:     conn.Spec.Host,
		Port:     conn.Spec.Port,
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
		Key: key,
		Data: map[string][]byte{
			conn.SecretKey(): []byte(dsn),
		},
	}

	var meta []*mariadbv1alpha1.Metadata
	if refs.MariaDB != nil && refs.MariaDB.Spec.InheritMetadata != nil {
		meta = append(meta, conn.Spec.SecretTemplate.Metadata)
	}
	if conn.Spec.SecretTemplate.Metadata != nil {
		meta = append(meta, conn.Spec.SecretTemplate.Metadata)
	}
	secretOpts.Metadata = meta

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
	if conn.Spec.HealthCheck == nil {
		return nil
	}
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
	patcher func(*mariadbv1alpha1.Connection) error) error {
	patch := client.MergeFrom(conn.DeepCopy())
	if err := patcher(conn); err != nil {
		return err
	}
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
