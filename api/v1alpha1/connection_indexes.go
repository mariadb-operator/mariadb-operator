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
	connectionPasswordSecretFieldPath      = ".spec.passwordSecretKeyRef.name"
	connectionTLSClientCertSecretFieldPath = ".spec.tlsClientCertSecretRef.name"
)

// IndexerFuncForFieldPath returns an indexer function for a given field path.
func (c *Connection) IndexerFuncForFieldPath(fieldPath string) (client.IndexerFunc, error) {
	switch fieldPath {
	case connectionPasswordSecretFieldPath:
		return func(obj client.Object) []string {
			connection, ok := obj.(*Connection)
			if !ok {
				return nil
			}
			if connection.Spec.PasswordSecretKeyRef != nil && connection.Spec.PasswordSecretKeyRef.Name != "" {
				return []string{connection.Spec.PasswordSecretKeyRef.Name}
			}
			return nil
		}, nil
	case connectionTLSClientCertSecretFieldPath:
		return func(obj client.Object) []string {
			connection, ok := obj.(*Connection)
			if !ok {
				return nil
			}
			if connection.Spec.TLSClientCertSecretRef != nil && connection.Spec.TLSClientCertSecretRef.Name != "" {
				return []string{connection.Spec.TLSClientCertSecretRef.Name}
			}
			return nil
		}, nil
	default:
		return nil, fmt.Errorf("unsupported field path: %s", fieldPath)
	}
}

// IndexConnection watches and indexes external resources referred by Connection resources.
func IndexConnection(ctx context.Context, mgr manager.Manager, builder *ctrlbuilder.Builder, client client.Client) error {
	watcherIndexer := watch.NewWatcherIndexer(mgr, builder, client)

	secretFieldPaths := []string{
		connectionPasswordSecretFieldPath,
		connectionTLSClientCertSecretFieldPath,
	}
	for _, fieldPath := range secretFieldPaths {
		if err := watcherIndexer.Watch(
			ctx,
			&corev1.Secret{},
			&Connection{},
			&ConnectionList{},
			fieldPath,
			ctrlbuilder.WithPredicates(
				predicate.PredicateWithLabel(metadata.WatchLabel),
			),
		); err != nil {
			return fmt.Errorf("error watching '%s': %v", fieldPath, err)
		}
	}

	return nil
}
