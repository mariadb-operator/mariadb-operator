package controller

import (
	"context"
	"fmt"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v25/api/v1alpha1"
	condition "github.com/mariadb-operator/mariadb-operator/v25/pkg/condition"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/controller/sql"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/refresolver"
	sqlClient "github.com/mariadb-operator/mariadb-operator/v25/pkg/sql"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// UserReconciler reconciles a User object
type UserReconciler struct {
	client.Client
	RefResolver    *refresolver.RefResolver
	ConditionReady *condition.Ready
	SqlOpts        []sql.SqlOpt
}

func NewUserReconciler(client client.Client, refResolver *refresolver.RefResolver, conditionReady *condition.Ready,
	sqlOpts ...sql.SqlOpt) *UserReconciler {
	return &UserReconciler{
		Client:         client,
		RefResolver:    refResolver,
		ConditionReady: conditionReady,
		SqlOpts:        sqlOpts,
	}
}

//+kubebuilder:rbac:groups=k8s.mariadb.com,resources=users,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=k8s.mariadb.com,resources=users/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=k8s.mariadb.com,resources=users/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *UserReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var user mariadbv1alpha1.User
	if err := r.Get(ctx, req.NamespacedName, &user); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	wr := newWrapperUserReconciler(r.Client, r.RefResolver, &user)
	wf := newWrappedUserFinalizer(r.Client, &user)
	tf := sql.NewSqlFinalizer(r.Client, wf, r.SqlOpts...)
	tr := sql.NewSqlReconciler(r.Client, r.ConditionReady, wr, tf, r.SqlOpts...)

	result, err := tr.Reconcile(ctx, &user)
	if err != nil {
		return result, fmt.Errorf("error reconciling in TemplateReconciler: %v", err)
	}
	return result, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *UserReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	builder := ctrl.NewControllerManagedBy(mgr).
		For(&mariadbv1alpha1.User{})

	if err := mariadbv1alpha1.IndexUser(ctx, mgr, builder, r.Client); err != nil {
		return fmt.Errorf("error indexing User: %v", err)
	}

	return builder.Complete(r)
}

type wrappedUserReconciler struct {
	client.Client
	refResolver *refresolver.RefResolver
	user        *mariadbv1alpha1.User
}

func newWrapperUserReconciler(client client.Client, refResolver *refresolver.RefResolver,
	user *mariadbv1alpha1.User) sql.WrappedReconciler {
	return &wrappedUserReconciler{
		Client:      client,
		refResolver: refResolver,
		user:        user,
	}
}

func (wr *wrappedUserReconciler) Reconcile(ctx context.Context, mdbClient *sqlClient.Client) error {
	var createUserOpts []sqlClient.CreateUserOpt

	var password string
	var passwordHash string
	var passwordVia string

	//nolint:nestif
	if wr.user.Spec.PasswordPlugin.PluginNameSecretKeyRef != nil {
		var err error
		passwordVia, err = wr.refResolver.SecretKeyRef(ctx, *wr.user.Spec.PasswordPlugin.PluginNameSecretKeyRef, wr.user.Namespace)
		if err != nil {
			return fmt.Errorf("error reading user password via secret: %v", err)
		}
		createUserOpts = append(createUserOpts, sqlClient.WithIdentifiedVia(passwordVia))

		var passwordViaUsing string
		if wr.user.Spec.PasswordPlugin.PluginArgSecretKeyRef != nil {
			var err error
			passwordViaUsing, err = wr.refResolver.SecretKeyRef(ctx, *wr.user.Spec.PasswordPlugin.PluginArgSecretKeyRef, wr.user.Namespace)
			if err != nil {
				return fmt.Errorf("error reading user password via using secret: %v", err)
			}
			createUserOpts = append(createUserOpts, sqlClient.WithIdentifiedViaUsing(passwordViaUsing))
		}

	} else if wr.user.Spec.PasswordHashSecretKeyRef != nil {
		var err error
		passwordHash, err = wr.refResolver.SecretKeyRef(ctx, *wr.user.Spec.PasswordHashSecretKeyRef, wr.user.Namespace)
		if err != nil {
			return fmt.Errorf("error reading user password hash secret: %v", err)
		}
		createUserOpts = append(createUserOpts, sqlClient.WithIdentifiedByPassword(passwordHash))
	} else if wr.user.Spec.PasswordSecretKeyRef != nil {
		var err error
		password, err = wr.refResolver.SecretKeyRef(ctx, *wr.user.Spec.PasswordSecretKeyRef, wr.user.Namespace)
		if err != nil {
			return fmt.Errorf("error reading user password secret: %v", err)
		}
		createUserOpts = append(createUserOpts, sqlClient.WithIdentifiedBy(password))
	}

	if wr.user.Spec.Require != nil {
		createUserOpts = append(createUserOpts, sqlClient.WithTLSRequirements(wr.user.Spec.Require))
	}

	createUserOpts = append(createUserOpts, sqlClient.WithMaxUserConnections(wr.user.Spec.MaxUserConnections))

	username := wr.user.UsernameOrDefault()
	hostname := wr.user.HostnameOrDefault()
	accountName := wr.user.AccountName()

	exists, err := mdbClient.UserExists(ctx, username, hostname)
	if err != nil {
		log.FromContext(ctx).Error(err, "Error checking if User exists")
	}

	if !exists {
		// This forces the user to be recreated from a clean state.
		// It helps fixing intermediate states in mysql.global_priv and mysql.user.
		if err := mdbClient.DropUser(ctx, accountName); err != nil {
			return fmt.Errorf("error dropping User: %v", err)
		}
		if err := mdbClient.CreateUser(ctx, accountName, createUserOpts...); err != nil {
			return fmt.Errorf("error creating User: %v", err)
		}
	} else if password != "" || passwordHash != "" || passwordVia != "" {
		if err := mdbClient.AlterUser(ctx, accountName, createUserOpts...); err != nil {
			return fmt.Errorf("error altering User: %v", err)
		}
	}
	return nil
}

func (wr *wrappedUserReconciler) PatchStatus(ctx context.Context, patcher condition.Patcher) error {
	patch := client.MergeFrom(wr.user.DeepCopy())
	patcher(&wr.user.Status)

	if err := wr.Client.Status().Patch(ctx, wr.user, patch); err != nil {
		return fmt.Errorf("error patching User status: %v", err)
	}
	return nil
}
