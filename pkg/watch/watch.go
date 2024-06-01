package watch

import (
	"context"
	"fmt"
	"reflect"

	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type Indexer interface {
	client.Object
	IndexerFuncForFieldPath(fieldPath string) (client.IndexerFunc, error)
}

type ItemLister interface {
	ctrlclient.ObjectList
	ListItems() []ctrlclient.Object
}

func NewItemListerOfType(itemLister ItemLister) ItemLister {
	itemType := reflect.TypeOf(itemLister).Elem()
	return reflect.New(itemType).Interface().(ItemLister)
}

type WatcherIndexer struct {
	mgr     ctrl.Manager
	builder *builder.Builder
	client  ctrlclient.Client
}

func NewWatcherIndexer(mgr ctrl.Manager, builder *builder.Builder, client ctrlclient.Client) *WatcherIndexer {
	return &WatcherIndexer{
		mgr:     mgr,
		builder: builder,
		client:  client,
	}
}

func (rw *WatcherIndexer) Watch(ctx context.Context, obj client.Object, indexer Indexer, indexerList ItemLister,
	indexerFieldPath string, opts ...builder.WatchesOption) error {

	indexerFn, err := indexer.IndexerFuncForFieldPath(indexerFieldPath)
	if err != nil {
		return fmt.Errorf("error getting indexer func: %v", err)
	}
	if err := rw.mgr.GetFieldIndexer().IndexField(ctx, indexer, indexerFieldPath, indexerFn); err != nil {
		return fmt.Errorf("error indexing '%s' field: %v", indexerFieldPath, err)
	}

	rw.builder.Watches(
		obj,
		handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, o ctrlclient.Object) []reconcile.Request {
			return rw.mapWatchedObjectToRequests(ctx, o, indexerList, indexerFieldPath)
		}),
		opts...,
	)
	return nil
}

func (rw *WatcherIndexer) mapWatchedObjectToRequests(ctx context.Context, obj ctrlclient.Object, indexList ItemLister,
	indexerFieldPath string) []reconcile.Request {
	indexersToReconcile := NewItemListerOfType(indexList)
	listOpts := &ctrlclient.ListOptions{
		FieldSelector: fields.OneTermEqualSelector(indexerFieldPath, obj.GetName()),
		Namespace:     obj.GetNamespace(),
	}

	if err := rw.client.List(ctx, indexersToReconcile, listOpts); err != nil {
		return []reconcile.Request{}
	}

	items := indexersToReconcile.ListItems()
	requests := make([]reconcile.Request, len(items))
	for i, item := range items {
		requests[i] = reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      item.GetName(),
				Namespace: item.GetNamespace(),
			},
		}
	}
	return requests
}
