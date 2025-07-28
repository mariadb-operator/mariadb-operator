package v1alpha1

import (
	"context"
	"fmt"

	"github.com/mariadb-operator/mariadb-operator/v25/pkg/metadata"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/predicate"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/watch"
	corev1 "k8s.io/api/core/v1"
	ctrlbuilder "sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

const (
	userPasswordSecretFieldPath           = ".spec.passwordSecretKeyRef.name"
	userPasswordHashSecretFieldPath       = ".spec.passwordHashSecretKeyRef.name"
	userPasswordPluginNameSecretFieldPath = ".spec.passwordPlugin.pluginNameSecretKeyRef.name"
	userPasswordPluginArgSecretFieldPath  = ".spec.passwordPlugin.pluginArgSecretKeyRef.name"
)

// IndexerFuncForFieldPath returns an indexer function for a given field path.
func (u *User) IndexerFuncForFieldPath(fieldPath string) (client.IndexerFunc, error) {
	switch fieldPath {
	case userPasswordSecretFieldPath:
		return func(obj client.Object) []string {
			user, ok := obj.(*User)
			if !ok {
				return nil
			}
			if user.Spec.PasswordSecretKeyRef != nil && user.Spec.PasswordSecretKeyRef.Name != "" {
				return []string{user.Spec.PasswordSecretKeyRef.Name}
			}
			return nil
		}, nil
	case userPasswordHashSecretFieldPath:
		return func(obj client.Object) []string {
			user, ok := obj.(*User)
			if !ok {
				return nil
			}
			if user.Spec.PasswordHashSecretKeyRef != nil && user.Spec.PasswordHashSecretKeyRef.Name != "" {
				return []string{user.Spec.PasswordHashSecretKeyRef.Name}
			}
			return nil
		}, nil
	case userPasswordPluginNameSecretFieldPath:
		return func(obj client.Object) []string {
			user, ok := obj.(*User)
			if !ok {
				return nil
			}
			if user.Spec.PasswordPlugin.PluginNameSecretKeyRef != nil &&
				user.Spec.PasswordPlugin.PluginNameSecretKeyRef.Name != "" {
				return []string{user.Spec.PasswordPlugin.PluginNameSecretKeyRef.Name}
			}
			return nil
		}, nil
	case userPasswordPluginArgSecretFieldPath:
		return func(obj client.Object) []string {
			user, ok := obj.(*User)
			if !ok {
				return nil
			}
			if user.Spec.PasswordPlugin.PluginArgSecretKeyRef != nil &&
				user.Spec.PasswordPlugin.PluginArgSecretKeyRef.Name != "" {
				return []string{user.Spec.PasswordPlugin.PluginArgSecretKeyRef.Name}
			}
			return nil
		}, nil
	default:
		return nil, fmt.Errorf("unsupported field path: %s", fieldPath)
	}
}

// IndexUser watches and indexes external resources referred by User resources.
func IndexUser(ctx context.Context, mgr manager.Manager, builder *ctrlbuilder.Builder, client client.Client) error {
	watcherIndexer := watch.NewWatcherIndexer(mgr, builder, client)

	secretFieldPaths := []string{
		userPasswordSecretFieldPath,
		userPasswordHashSecretFieldPath,
		userPasswordPluginNameSecretFieldPath,
		userPasswordPluginArgSecretFieldPath,
	}
	for _, fieldPath := range secretFieldPaths {
		if err := watcherIndexer.Watch(
			ctx,
			&corev1.Secret{},
			&User{},
			&UserList{},
			fieldPath,
			ctrlbuilder.WithPredicates(
				predicate.PredicateWithLabel(metadata.WatchLabel),
			),
		); err != nil {
			return fmt.Errorf("error watching: %v", err)
		}
	}

	return nil
}
