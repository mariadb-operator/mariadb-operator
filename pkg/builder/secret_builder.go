package builder

import (
	"fmt"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	metadata "github.com/mariadb-operator/mariadb-operator/pkg/builder/metadata"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type SecretOpts struct {
	Metadata []*mariadbv1alpha1.Metadata
	Key      types.NamespacedName
	Data     map[string][]byte
}

func (b *Builder) BuildSecret(opts SecretOpts, owner metav1.Object) (*corev1.Secret, error) {
	objMetaBuilder :=
		metadata.NewMetadataBuilder(opts.Key)
	for _, meta := range opts.Metadata {
		objMetaBuilder = objMetaBuilder.WithMetadata(meta)
	}
	objMeta := objMetaBuilder.Build()

	secret := &corev1.Secret{
		ObjectMeta: objMeta,
		Data:       opts.Data,
	}
	if err := controllerutil.SetControllerReference(owner, secret, b.scheme); err != nil {
		return nil, fmt.Errorf("error setting controller reference to Secret: %v", err)
	}
	return secret, nil
}
