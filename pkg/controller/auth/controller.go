package auth

import (
	"context"
	"fmt"
	"time"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v25/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/builder"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/controller/secret"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/refresolver"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type AuthReconciler struct {
	client.Client
	refResolver      *refresolver.RefResolver
	builder          *builder.Builder
	secretReconciler *secret.SecretReconciler
}

func NewAuthReconciler(client client.Client, builder *builder.Builder, secretReconciler *secret.SecretReconciler) *AuthReconciler {
	return &AuthReconciler{
		Client:           client,
		refResolver:      refresolver.New(client),
		secretReconciler: secretReconciler,
		builder:          builder,
	}
}

type GrantOpts struct {
	builder.GrantOpts
	Key types.NamespacedName
}

// NamespacedOwner is used when creating a user password if it does not exist
type NamespacedOwner interface {
	client.Object
	GetInheritMetadata() *mariadbv1alpha1.Metadata
}

// reconcileRequest is an internal object that is used within the ReconcileUserGrant
type reconcileRequest struct {
	WaitForUser   bool
	WaitForGrants bool
	owner         NamespacedOwner
	secretKeyRef  *mariadbv1alpha1.GeneratedSecretKeyRef
}

func (r *reconcileRequest) isGeneratePassword() bool {
	return r.owner != nil && r.secretKeyRef != nil
}

type ReconciliationOpts func(req *reconcileRequest) error

func WithWaitForUser(waitForUser bool) ReconciliationOpts {
	return func(req *reconcileRequest) error {
		req.WaitForUser = waitForUser
		return nil
	}
}

func WithWaitForGrant(waitForGrants bool) ReconciliationOpts {
	return func(req *reconcileRequest) error {
		req.WaitForGrants = waitForGrants
		return nil
	}
}

// WithGeneratePassword specifies the owner and metadata provider for the secret as well as the ref to use.
func WithGeneratePassword(owner NamespacedOwner, secretKeyRef *mariadbv1alpha1.GeneratedSecretKeyRef) ReconciliationOpts {
	return func(req *reconcileRequest) error {
		req.owner = owner
		req.secretKeyRef = secretKeyRef
		return nil
	}
}

// ReconcileUserGrant will reconcile a user and a grant. This involves multiple requeues
// Defaults:
// Waits for user creation
// Doesn't wait for Grant creation
// Does not generate password
func (r *AuthReconciler) ReconcileUserGrant(ctx context.Context, key types.NamespacedName, owner metav1.Object,
	userOpts builder.UserOpts, grantOpts []GrantOpts, reconcOpts ...ReconciliationOpts) (ctrl.Result, error) {
	recRequest := &reconcileRequest{
		WaitForUser:   true,
		WaitForGrants: false,
		owner:         nil,
		secretKeyRef:  nil,
	}
	for _, opt := range reconcOpts {
		if err := opt(recRequest); err != nil {
			return ctrl.Result{}, fmt.Errorf("error applying reconcile options: %v", err)
		}
	}

	if recRequest.isGeneratePassword() {
		if err := r.ReconcileUserPassword(ctx, recRequest.owner, recRequest.secretKeyRef); err != nil {
			return ctrl.Result{}, err
		}
	}

	if err := r.ReconcileUser(ctx, key, owner, userOpts); err != nil {
		return ctrl.Result{}, fmt.Errorf("error reconciling User: %v", err)
	}

	if recRequest.WaitForUser {
		if result, err := r.WaitForUser(ctx, key); !result.IsZero() || err != nil {
			return result, err
		}
	}

	for _, gops := range grantOpts {
		if len(gops.Privileges) > 0 {
			if err := r.ReconcileGrant(ctx, gops.Key, key, owner, gops.GrantOpts); err != nil {
				return ctrl.Result{}, fmt.Errorf("error reconciling Grant: %v", err)
			}

			if recRequest.WaitForGrants {
				if result, err := r.WaitForGrant(ctx, gops.Key); !result.IsZero() || err != nil {
					return result, err
				}
			}
		}
	}

	return ctrl.Result{}, nil
}

func (r *AuthReconciler) ReconcileUser(ctx context.Context, key types.NamespacedName, owner metav1.Object,
	userOpts builder.UserOpts) error {
	var user mariadbv1alpha1.User
	err := r.Get(ctx, key, &user)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.FromContext(ctx).V(1).Info("Creating User", "key", key, "owner", owner, "opts", userOpts)
			return r.createUser(ctx, key, owner, userOpts)
		}
		return err
	}
	return nil
}

func (r *AuthReconciler) ReconcileGrant(ctx context.Context, key, userKey types.NamespacedName, owner metav1.Object,
	grantOpts builder.GrantOpts) error {
	var user mariadbv1alpha1.User
	if err := r.Get(ctx, userKey, &user); err != nil {
		return err
	}

	var grant mariadbv1alpha1.Grant
	if err := r.Get(ctx, key, &grant); err != nil {
		if apierrors.IsNotFound(err) {
			log.FromContext(ctx).V(1).Info("Creating User Grant", "key", key, "owner", owner, "opts", grantOpts)
			return r.createGrant(ctx, key, owner, grantOpts)
		}
		return err
	}
	return nil
}

// waitForGrant allows us to wait for a Grant resource to be created.
// Requeue accordingly to the result
func (r *AuthReconciler) WaitForGrant(ctx context.Context, key types.NamespacedName) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var grant mariadbv1alpha1.Grant
	if err := r.Get(ctx, key, &grant); err != nil {
		if apierrors.IsNotFound(err) {
			logger.V(1).Info("Grant not found. Requeuing", "grant", key.Name)
			return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
		}

		return ctrl.Result{}, err
	}

	if !grant.IsReady() {
		logger.V(1).Info("Grant not ready. Requeuing...", "grant", key.Name)
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	return ctrl.Result{}, nil
}

func (r *AuthReconciler) WaitForUser(ctx context.Context, key types.NamespacedName) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var user mariadbv1alpha1.User
	if err := r.Get(ctx, key, &user); err != nil {
		if apierrors.IsNotFound(err) {
			logger.V(1).Info("User not found. Requeuing", "user", key.Name)
			return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
		}
		return ctrl.Result{}, err
	}

	if !user.IsReady() {
		logger.V(1).Info("User not ready. Requeuing", "user", key.Name)
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}
	return ctrl.Result{}, nil
}

func (r *AuthReconciler) createUser(ctx context.Context, key types.NamespacedName, owner metav1.Object,
	userOpts builder.UserOpts) error {
	user, err := r.builder.BuildUser(key, owner, userOpts)
	if err != nil {
		return fmt.Errorf("error building User: %v", err)
	}
	return r.Create(ctx, user)
}

func (r *AuthReconciler) createGrant(ctx context.Context, key types.NamespacedName, owner metav1.Object,
	grantOpts builder.GrantOpts) error {
	user, err := r.builder.BuildGrant(key, owner, grantOpts)
	if err != nil {
		return fmt.Errorf("error building Grant: %v", err)
	}
	return r.Create(ctx, user)
}

// ReconcileUserPassword will create a new secret with the user password if it does not already exists
func (a *AuthReconciler) ReconcileUserPassword(ctx context.Context, owner NamespacedOwner, secretKeyRef *mariadbv1alpha1.GeneratedSecretKeyRef) error {
	req := secret.PasswordRequest{
		Metadata: owner.GetInheritMetadata(),
		Owner:    owner,
		Key: types.NamespacedName{
			Name:      secretKeyRef.Name,
			Namespace: owner.GetNamespace(),
		},
		SecretKey: secretKeyRef.Key,
		Generate:  secretKeyRef.Generate,
	}
	_, err := a.secretReconciler.ReconcilePassword(ctx, req)
	return err
}
