package builder

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type ConfigMapOpts struct {
	Key  types.NamespacedName
	Data map[string]string
}

func (b *Builder) BuildConfigMap(opts ConfigMapOpts, owner metav1.Object) (*corev1.ConfigMap, error) {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      opts.Key.Name,
			Namespace: opts.Key.Namespace,
		},
		Data: opts.Data,
	}
	if err := controllerutil.SetControllerReference(owner, cm, b.scheme); err != nil {
		return nil, fmt.Errorf("error setting controller reference to ConfigMap: %v", err)
	}
	return cm, nil
}
