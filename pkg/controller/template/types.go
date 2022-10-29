package template

import (
	"context"

	databasev1alpha1 "github.com/mmontes11/mariadb-operator/api/v1alpha1"
	"github.com/mmontes11/mariadb-operator/pkg/conditions"
	mariadbclient "github.com/mmontes11/mariadb-operator/pkg/mariadb"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

type Resource interface {
	Meta() metav1.ObjectMeta
	MariaDBRef() *databasev1alpha1.MariaDBRef
	IsBeingDeleted() bool
}

type Reconciler interface {
	Reconcile(ctx context.Context, resource Resource) (ctrl.Result, error)
}

type WrappedReconciler interface {
	Reconcile(context.Context, *mariadbclient.Client) error
	PatchStatus(context.Context, conditions.Patcher) error
}

type Finalizer interface {
	AddFinalizer(context.Context) error
	Finalize(context.Context, Resource) error
}

type WrappedFinalizer interface {
	AddFinalizer(context.Context) error
	RemoveFinalizer(context.Context) error
	ContainsFinalizer() bool
	Reconcile(context.Context, *mariadbclient.Client) error
}
