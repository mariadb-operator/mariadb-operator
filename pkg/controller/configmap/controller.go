package configmap

import (
	"context"
	"fmt"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/pkg/builder"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ConfigMapReconciler struct {
	client.Client
	Builder *builder.Builder
}

func NewConfigMapReconciler(client client.Client, builder *builder.Builder) *ConfigMapReconciler {
	return &ConfigMapReconciler{
		Client:  client,
		Builder: builder,
	}
}

type ReconcileRequest struct {
	Metadata *mariadbv1alpha1.Metadata
	Owner    metav1.Object
	Key      types.NamespacedName
	Data     map[string]string
}

func (r *ConfigMapReconciler) Reconcile(ctx context.Context, req *ReconcileRequest) error {
	opts := builder.ConfigMapOpts{
		Metadata: req.Metadata,
		Key:      req.Key,
		Data:     req.Data,
	}
	configMap, err := r.Builder.BuildConfigMap(opts, req.Owner)
	if err != nil {
		return fmt.Errorf("error building ConfigMap: %v", err)
	}

	var existingConfigMap corev1.ConfigMap
	if err := r.Get(ctx, req.Key, &existingConfigMap); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("error getting ConfigMap: %v", err)
		}
		if err := r.Create(ctx, configMap); err != nil {
			return fmt.Errorf("error creating ConfigMap: %v", err)
		}
		return nil
	}

	patch := client.MergeFrom(existingConfigMap.DeepCopy())
	existingConfigMap.Data = configMap.Data
	return r.Patch(ctx, &existingConfigMap, patch)
}
