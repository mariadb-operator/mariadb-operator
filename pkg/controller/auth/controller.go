package auth

import (
	"context"
	"fmt"
	"time"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/pkg/builder"
	"github.com/mariadb-operator/mariadb-operator/pkg/refresolver"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type AuthReconciler struct {
	client.Client
	refResolver *refresolver.RefResolver
	builder     *builder.Builder
}

func NewAuthReconciler(client client.Client, builder *builder.Builder) *AuthReconciler {
	return &AuthReconciler{
		Client:      client,
		refResolver: refresolver.New(client),
		builder:     builder,
	}
}

type GrantOpts struct {
	builder.GrantOpts
	Key types.NamespacedName
}

func (r *AuthReconciler) ReconcileUserGrant(ctx context.Context, key types.NamespacedName, owner metav1.Object,
	userOpts builder.UserOpts, grantOpts ...GrantOpts) (ctrl.Result, error) {
	if err := r.ReconcileUser(ctx, key, owner, userOpts); err != nil {
		return ctrl.Result{}, fmt.Errorf("error reconciling User: %v", err)
	}
	if result, err := r.waitForUser(ctx, key); !result.IsZero() || err != nil {
		return result, err
	}
	for _, gops := range grantOpts {
		if len(gops.Privileges) > 0 {
			if err := r.ReconcileGrant(ctx, gops.Key, key, owner, gops.GrantOpts); err != nil {
				return ctrl.Result{}, fmt.Errorf("error reconciling Grant: %v", err)
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
			return r.createGrant(ctx, key, owner, grantOpts)
		}
		return err
	}
	return nil
}

func (r *AuthReconciler) createUser(ctx context.Context, key types.NamespacedName, owner metav1.Object,
	userOpts builder.UserOpts) error {
	user, err := r.builder.BuildUser(key, owner, userOpts)
	if err != nil {
		return fmt.Errorf("error building User: %v", err)
	}
	return r.Client.Create(ctx, user)
}

func (r *AuthReconciler) waitForUser(ctx context.Context, key types.NamespacedName) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var user mariadbv1alpha1.User
	if err := r.Get(ctx, key, &user); err != nil {
		if apierrors.IsNotFound(err) {
			logger.V(1).Info("User not found. Requeuing", "user", key.Name)
			return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
		}
		return ctrl.Result{}, err
	}

	if !user.IsReady() {
		logger.V(1).Info("User not ready. Requeuing", "user", key.Name)
		return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
	}
	return ctrl.Result{}, nil
}

func (r *AuthReconciler) createGrant(ctx context.Context, key types.NamespacedName, owner metav1.Object,
	grantOpts builder.GrantOpts) error {
	user, err := r.builder.BuildGrant(key, owner, grantOpts)
	if err != nil {
		return fmt.Errorf("error building Grant: %v", err)
	}
	return r.Client.Create(ctx, user)
}
