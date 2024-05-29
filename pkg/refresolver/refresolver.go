package refresolver

import (
	"context"
	"errors"
	"fmt"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/pkg/metadata"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	ErrMariaDBAnnotationNotFound = errors.New("MariaDB annotation not found")
)

type RefResolver struct {
	client client.Client
}

func New(client client.Client) *RefResolver {
	return &RefResolver{
		client: client,
	}
}

func (r *RefResolver) MariaDB(ctx context.Context, ref *mariadbv1alpha1.MariaDBRef,
	namespace string) (*mariadbv1alpha1.MariaDB, error) {
	if ref.Kind != "" && ref.Kind != "MariaDB" {
		return nil, fmt.Errorf("Unsupported reference kind: '%s'", ref.Kind)
	}

	key := types.NamespacedName{
		Name:      ref.Name,
		Namespace: namespace,
	}
	if ref.Namespace != "" {
		key.Namespace = ref.Namespace
	}

	var mariadb mariadbv1alpha1.MariaDB
	if err := r.client.Get(ctx, key, &mariadb); err != nil {
		return nil, err
	}
	return &mariadb, nil
}

func (r *RefResolver) MariaDBFromAnnotation(ctx context.Context, objMeta metav1.ObjectMeta) (*mariadbv1alpha1.MariaDB, error) {
	mariadbAnnotation, ok := objMeta.Annotations[metadata.MariadbAnnotation]
	if !ok {
		return nil, ErrMariaDBAnnotationNotFound
	}

	var mariadb mariadbv1alpha1.MariaDB
	key := types.NamespacedName{
		Name:      mariadbAnnotation,
		Namespace: objMeta.Namespace,
	}
	if err := r.client.Get(ctx, key, &mariadb); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, err
		}
		return nil, fmt.Errorf("error getting MariaDB from annotation '%s': %v", objMeta.Name, err)
	}
	return &mariadb, nil
}

func (r *RefResolver) MaxScale(ctx context.Context, ref *corev1.ObjectReference,
	namespace string) (*mariadbv1alpha1.MaxScale, error) {
	if ref.Kind != "" && ref.Kind != "MaxScale" {
		return nil, fmt.Errorf("Unsupported reference kind: '%s'", ref.Kind)
	}

	key := types.NamespacedName{
		Name:      ref.Name,
		Namespace: namespace,
	}
	if ref.Namespace != "" {
		key.Namespace = ref.Namespace
	}

	var mxs mariadbv1alpha1.MaxScale
	if err := r.client.Get(ctx, key, &mxs); err != nil {
		return nil, err
	}
	return &mxs, nil
}

func (r *RefResolver) Backup(ctx context.Context, ref *corev1.LocalObjectReference,
	namespace string) (*mariadbv1alpha1.Backup, error) {
	nn := types.NamespacedName{
		Name:      ref.Name,
		Namespace: namespace,
	}
	var backup mariadbv1alpha1.Backup
	if err := r.client.Get(ctx, nn, &backup); err != nil {
		return nil, err
	}
	return &backup, nil
}

func (r *RefResolver) SqlJob(ctx context.Context, ref *corev1.LocalObjectReference,
	namespace string) (*mariadbv1alpha1.SqlJob, error) {
	nn := types.NamespacedName{
		Name:      ref.Name,
		Namespace: namespace,
	}
	var sqlJob mariadbv1alpha1.SqlJob
	if err := r.client.Get(ctx, nn, &sqlJob); err != nil {
		return nil, err
	}
	return &sqlJob, nil
}

func (r *RefResolver) SecretKeyRef(ctx context.Context, selector corev1.SecretKeySelector,
	namespace string) (string, error) {
	key := types.NamespacedName{
		Name:      selector.Name,
		Namespace: namespace,
	}
	var secret corev1.Secret
	if err := r.client.Get(ctx, key, &secret); err != nil {
		return "", fmt.Errorf("error getting Secret: %v", err)
	}

	data, ok := secret.Data[selector.Key]
	if !ok {
		return "", fmt.Errorf("Secret key \"%s\" not found", selector.Key)
	}
	return string(data), nil
}

func (r *RefResolver) ConfigMapKeyRef(ctx context.Context, selector *corev1.ConfigMapKeySelector,
	namespace string) (string, error) {
	key := types.NamespacedName{
		Name:      selector.Name,
		Namespace: namespace,
	}
	var configMap corev1.ConfigMap
	if err := r.client.Get(ctx, key, &configMap); err != nil {
		return "", fmt.Errorf("error getting ConfigMap: %v", err)
	}

	data, ok := configMap.Data[selector.Key]
	if !ok {
		return "", fmt.Errorf("ConfigMap key \"%s\" not found", selector.Key)
	}
	return string(data), nil
}
