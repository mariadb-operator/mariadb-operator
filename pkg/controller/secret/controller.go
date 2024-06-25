package secret

import (
	"context"
	"fmt"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/pkg/builder"
	"github.com/sethvargo/go-password/password"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type SecretReconciler struct {
	client.Client
	Builder   *builder.Builder
	generator *password.Generator
}

func NewSecretReconciler(client client.Client, builder *builder.Builder) (*SecretReconciler, error) {
	generator, err := password.NewGenerator(&password.GeneratorInput{
		Symbols: "~!@#$%^&*()_+-={}|[]:<>/",
	})
	if err != nil {
		return nil, fmt.Errorf("error creating password generator: %v", err)
	}

	return &SecretReconciler{
		Client:    client,
		Builder:   builder,
		generator: generator,
	}, nil
}

type PasswordRequest struct {
	Owner     metav1.Object
	Metadata  *mariadbv1alpha1.Metadata
	Key       types.NamespacedName
	SecretKey string
	Generate  bool
}

func (r *SecretReconciler) ReconcilePassword(ctx context.Context, req PasswordRequest) (string, error) {
	var existingSecret corev1.Secret
	err := r.Get(ctx, req.Key, &existingSecret)

	if err == nil {
		return string(existingSecret.Data[req.SecretKey]), nil
	}
	if !req.Generate {
		return "", fmt.Errorf("error reconciling password Secret: %v", err)
	}
	if !apierrors.IsNotFound(err) {
		return "", fmt.Errorf("error reconciling password Secret: %v", err)
	}

	password, err := r.generator.Generate(16, 4, 2, false, false)
	if err != nil {
		return "", fmt.Errorf("error generating password Secret: %v", err)
	}

	opts := builder.SecretOpts{
		Metadata: []*mariadbv1alpha1.Metadata{req.Metadata},
		Key:      req.Key,
		Data: map[string][]byte{
			req.SecretKey: []byte(password),
		},
	}
	secret, err := r.Builder.BuildSecret(opts, req.Owner)
	if err != nil {
		return "", fmt.Errorf("error building password Secret: %v", err)
	}
	if err := r.Create(ctx, secret); err != nil {
		return "", fmt.Errorf("error creating password Secret: %v", err)
	}

	return password, nil
}

type SecretRequest struct {
	Owner    metav1.Object
	Metadata []*mariadbv1alpha1.Metadata
	Key      types.NamespacedName
	Data     map[string][]byte
}

func (r *SecretReconciler) Reconcile(ctx context.Context, req *SecretRequest) error {
	secretOpts := builder.SecretOpts{
		Metadata: req.Metadata,
		Key:      req.Key,
		Data:     req.Data,
	}
	secret, err := r.Builder.BuildSecret(secretOpts, req.Owner)
	if err != nil {
		return fmt.Errorf("error building Secret: %v", err)
	}

	var existingSecret corev1.Secret
	if err := r.Get(ctx, req.Key, &existingSecret); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("error reconciling Secret: %v", err)
		}
		return r.Create(ctx, secret)
	}

	patch := client.MergeFrom(existingSecret.DeepCopy())
	existingSecret.Data = secret.Data
	return r.Patch(ctx, &existingSecret, patch)
}
