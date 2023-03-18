package configmap

import (
	"context"
	"fmt"

	"github.com/mariadb-operator/mariadb-operator/pkg/builder"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ConfigMapper interface {
	v1.Object
	ConfigMapValue() *string
	ConfigMapKeyRef() *corev1.ConfigMapKeySelector
}

type ConfigMapReconciler struct {
	client.Client
	Builder      *builder.Builder
	ConfigMapKey string
}

func NewConfigMapReconciler(client client.Client, builder *builder.Builder, configMapKey string) *ConfigMapReconciler {
	return &ConfigMapReconciler{
		Client:       client,
		Builder:      builder,
		ConfigMapKey: configMapKey,
	}
}

func (r *ConfigMapReconciler) Reconcile(ctx context.Context, configMapper ConfigMapper, key types.NamespacedName) error {

	if configMapper.ConfigMapValue() == nil && configMapper.ConfigMapKeyRef() == nil {
		return nil
	}

	if configMapper.ConfigMapKeyRef() != nil {
		var configMap corev1.ConfigMap
		key := types.NamespacedName{
			Name:      configMapper.ConfigMapKeyRef().Name,
			Namespace: key.Namespace,
		}
		if err := r.Get(ctx, key, &configMap); err != nil {
			return fmt.Errorf("error getting ConfigMap: %v", err)
		}
		return nil
	}

	opts := builder.ConfigMapOpts{
		Key: key,
		Data: map[string]string{
			r.ConfigMapKey: *configMapper.ConfigMapValue(),
		},
	}
	configMap, err := r.Builder.BuildConfigMap(opts, configMapper)
	if err != nil {
		return fmt.Errorf("error building ConfigMap: %v", err)
	}

	if err = r.Create(ctx, configMap); err != nil {
		return fmt.Errorf("error creating ConfigMap: %v", err)
	}
	return nil
}
