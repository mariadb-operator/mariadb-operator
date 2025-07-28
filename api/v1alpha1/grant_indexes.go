package v1alpha1

import (
	"context"
	"fmt"

	"github.com/mariadb-operator/mariadb-operator/v25/pkg/watch"
	ctrlbuilder "sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

const grantUsernameFieldPath = ".spec.username"

// IndexerFuncForFieldPath returns an indexer function for a given field path.
func (g *Grant) IndexerFuncForFieldPath(fieldPath string) (client.IndexerFunc, error) {
	switch fieldPath {
	case grantUsernameFieldPath:
		return func(obj client.Object) []string {
			grant, ok := obj.(*Grant)
			if !ok {
				return nil
			}
			if grant.Spec.Username != "" {
				return []string{grant.Spec.Username}
			}
			return nil
		}, nil
	default:
		return nil, fmt.Errorf("unsupported field path: %s", fieldPath)
	}
}

// IndexGrant watches and indexes external resources referred by Grant resources.
func IndexGrant(ctx context.Context, mgr manager.Manager, builder *ctrlbuilder.Builder, client client.Client) error {
	watcherIndexer := watch.NewWatcherIndexer(mgr, builder, client)

	if err := watcherIndexer.Watch(
		ctx,
		&User{},
		&Grant{},
		&GrantList{},
		grantUsernameFieldPath,
		ctrlbuilder.WithPredicates(predicate.Funcs{
			CreateFunc: func(ce event.CreateEvent) bool {
				return true
			},
		}),
	); err != nil {
		return fmt.Errorf("error watching: %v", err)
	}

	return nil
}
