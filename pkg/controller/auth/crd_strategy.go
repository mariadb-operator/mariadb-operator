package auth

import (
	"context"
	"fmt"
	"time"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v25/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/builder"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/controller/secret"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// NamespacedOwner is used when creating a user password if it does not exist
type NamespacedOwner interface {
	client.Object
	GetInheritMetadata() *mariadbv1alpha1.Metadata
}

type CrdStrategyOpts func(strategy *CrdStrategy) error

// CrdStrategy is responsible for creating CRD users and grants, leaving the functionality to the
// user and grants controller.
type CrdStrategy struct {
	client.Client
	builder          *builder.Builder
	secretReconciler *secret.SecretReconciler
	owner            NamespacedOwner
	secretKeyRef     *mariadbv1alpha1.GeneratedSecretKeyRef
	userKey          types.NamespacedName
	grantKeys        []types.NamespacedName

	WaitForUser   bool
	WaitForGrants bool
}

// NewCrdStrategy is used to setup some defaults.
func NewCrdStrategy(apiClient client.Client, builder *builder.Builder, opts ...CrdStrategyOpts) (*CrdStrategy, error) {
	strategy := &CrdStrategy{
		Client:        apiClient,
		builder:       builder,
		WaitForUser:   true, // Backwards compat
		WaitForGrants: false,
	}

	for _, opt := range opts {
		if err := opt(strategy); err != nil {
			return nil, fmt.Errorf("error applying CrdStrategy options: %w", err)
		}
	}

	return strategy, nil
}

// WithWait is optional. If not given, will wait only for user (for backwards compatibility)
func WithWait(waitForUser bool, waitForGrants bool) CrdStrategyOpts {
	return func(strategy *CrdStrategy) error {
		strategy.WaitForUser = waitForUser
		strategy.WaitForGrants = waitForGrants
		return nil
	}
}

// WithUserKeys is needed. We specify the keys of the User CR
func WithUserKeys(userKey types.NamespacedName) CrdStrategyOpts {
	return func(strategy *CrdStrategy) error {
		strategy.userKey = userKey
		return nil
	}
}

// WithGrantKeys is needed if we have grants. We specify the keys **in order** of the Grant CR
func WithGrantKeys(grantKeys ...types.NamespacedName) CrdStrategyOpts {
	return func(strategy *CrdStrategy) error {
		strategy.grantKeys = grantKeys
		return nil
	}
}

// WithOwner is needed to specify the owner of the CRDs
func WithOwner(owner NamespacedOwner) CrdStrategyOpts {
	return func(strategy *CrdStrategy) error {
		strategy.owner = owner
		return nil
	}
}

// WithSecretKeyRef essentially means that we will use the password from a secret.
// This gives you the ability to generate a custom password if needed.
// If you wish to do so, provide the secretReconciler
func WithSecretKeyRef(secretKeyref *mariadbv1alpha1.GeneratedSecretKeyRef, secretReconciler *secret.SecretReconciler) CrdStrategyOpts {
	return func(strategy *CrdStrategy) error {
		strategy.secretReconciler = secretReconciler
		strategy.secretKeyRef = secretKeyref
		return nil
	}
}

func (s *CrdStrategy) isReconcilePassword() bool {
	return s.secretKeyRef != nil && s.secretReconciler != nil
}

func (s *CrdStrategy) reconcileUser(ctx context.Context, userOpts builder.UserOpts) (ctrl.Result, error) {
	if s.userKey == (types.NamespacedName{}) || s.owner == nil {
		return ctrl.Result{}, fmt.Errorf("userKey or owner is not specified when reconciling user")
	}
	key := s.userKey

	if s.isReconcilePassword() {
		if _, err := s.reconcileUserPassword(ctx); err != nil {
			return ctrl.Result{}, err
		}
	}

	if err := s.createUser(ctx, key, userOpts); err != nil {
		return ctrl.Result{}, err
	}

	if result, err := s.waitForUser(ctx, userOpts); !result.IsZero() || err != nil {
		return result, err
	}
	return ctrl.Result{}, nil
}

// createUser will create the `User` CustomResource
func (s *CrdStrategy) createUser(ctx context.Context, key types.NamespacedName, userOpts builder.UserOpts) error {
	var user mariadbv1alpha1.User
	err := s.Get(ctx, key, &user)
	if apierrors.IsNotFound(err) {
		user, err := s.builder.BuildUser(key, s.owner, userOpts)
		if err != nil {
			return fmt.Errorf("error building User: %w", err)
		}
		return s.Create(ctx, user)
	}

	return err
}

func (s *CrdStrategy) waitForUser(ctx context.Context, userOpts builder.UserOpts) (ctrl.Result, error) {
	key := s.userKey

	logger := log.FromContext(ctx)

	var user mariadbv1alpha1.User
	if err := s.Get(ctx, key, &user); err != nil {
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

// reconcileGrant will create a new grant and potentially wait for it.
func (s *CrdStrategy) reconcileGrant(ctx context.Context, userOpts builder.UserOpts, grantOpts builder.GrantOpts) (ctrl.Result, error) {
	var grantKey types.NamespacedName
	if len(s.grantKeys) == 0 {
		return ctrl.Result{}, fmt.Errorf("error getting Grant key for grant. Not enough grantKeys given")
	}
	grantKey, s.grantKeys = s.grantKeys[0], s.grantKeys[1:]

	if grantKey == (types.NamespacedName{}) || s.owner == nil {
		return ctrl.Result{}, fmt.Errorf("grantKey or owner is not specified when reconciling user")
	}

	if err := s.createGrant(ctx, grantKey, grantOpts); err != nil {
		return ctrl.Result{}, err
	}

	if result, err := s.waitForGrant(ctx, grantKey); !result.IsZero() || err != nil {
		return result, err
	}

	return ctrl.Result{}, nil
}

func (s *CrdStrategy) createGrant(ctx context.Context, key types.NamespacedName, grantOpts builder.GrantOpts) error {
	var grant mariadbv1alpha1.Grant
	err := s.Get(ctx, key, &grant)

	if apierrors.IsNotFound(err) {
		user, err := s.builder.BuildGrant(key, s.owner, grantOpts)
		if err != nil {
			return fmt.Errorf("error building Grant: %w", err)
		}
		return s.Create(ctx, user)
	}

	return err
}

// ReconcileUserPassword will create a new secret with the user password if it does not already exists
func (s *CrdStrategy) reconcileUserPassword(ctx context.Context) (string, error) {
	req := secret.PasswordRequest{
		Metadata: s.owner.GetInheritMetadata(),
		Owner:    s.owner,
		Key: types.NamespacedName{
			Name:      s.secretKeyRef.Name,
			Namespace: s.owner.GetNamespace(),
		},
		SecretKey: s.secretKeyRef.Key,
		Generate:  s.secretKeyRef.Generate,
	}
	return s.secretReconciler.ReconcilePassword(ctx, req)
}

// waitForGrant allows us to wait for a Grant resource to be created.
// Requeue accordingly to the result
func (s *CrdStrategy) waitForGrant(ctx context.Context, grantKey types.NamespacedName) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var grant mariadbv1alpha1.Grant
	if err := s.Get(ctx, grantKey, &grant); err != nil {
		if apierrors.IsNotFound(err) {
			logger.V(1).Info("Grant not found. Requeuing", "grant", grantKey.Name)
			return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
		}

		return ctrl.Result{}, err
	}

	if !grant.IsReady() {
		logger.V(1).Info("Grant not ready. Requeuing...", "grant", grantKey.Name)
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	return ctrl.Result{}, nil
}
