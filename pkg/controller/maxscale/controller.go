package maxscale

import (
	"context"
	"fmt"
	"reflect"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/pkg/builder"
	"github.com/mariadb-operator/mariadb-operator/pkg/environment"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type MaxScaleReconciler struct {
	client.Client
	builder     *builder.Builder
	environment *environment.OperatorEnv
}

func NewMaxScaleReconciler(client client.Client, builder *builder.Builder, env *environment.OperatorEnv) *MaxScaleReconciler {
	return &MaxScaleReconciler{
		Client:      client,
		builder:     builder,
		environment: env,
	}
}

func (r *MaxScaleReconciler) Reconcile(ctx context.Context, mdb *mariadbv1alpha1.MariaDB) (ctrl.Result, error) {
	if !ptr.Deref(mdb.Spec.MaxScale, mariadbv1alpha1.MariaDBMaxScaleSpec{}).Enabled {
		return ctrl.Result{}, nil
	}

	key := mdb.MaxScaleKey()
	desiredMxs, err := r.builder.BuildMaxScale(key, mdb, mdb.Spec.MaxScale)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error building MaxScale: %v", err)
	}

	var existingMxs mariadbv1alpha1.MaxScale
	if err := r.Get(ctx, key, &existingMxs); err != nil {
		if !apierrors.IsNotFound(err) {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, r.Create(ctx, desiredMxs)
	}

	patch := client.MergeFrom(existingMxs.DeepCopy())

	existingSpec := &existingMxs.Spec
	desiredSpec := &desiredMxs.Spec
	specType := reflect.TypeOf(*existingSpec)
	specValue := reflect.ValueOf(existingSpec).Elem()

	for i := 0; i < specType.NumField(); i++ {
		existingFieldValue := specValue.Field(i)
		desiredFieldValue := reflect.ValueOf(desiredSpec).Elem().Field(i)

		if !desiredFieldValue.IsZero() && existingFieldValue.IsZero() {
			existingFieldValue.Set(desiredFieldValue)
		}
	}

	return ctrl.Result{}, r.Patch(ctx, &existingMxs, patch)
}
