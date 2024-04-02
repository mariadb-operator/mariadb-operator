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

type ConnectionOpts struct {
	Metadata             *mariadbv1alpha1.Metadata
	MariaDB              *mariadbv1alpha1.MariaDB
	MaxScale             *mariadbv1alpha1.MaxScale
	Key                  types.NamespacedName
	Username             string
	PasswordSecretKeyRef corev1.SecretKeySelector
	Database             *string
	Template             *mariadbv1alpha1.ConnectionTemplate
}

func (b *Builder) BuildConnection(opts ConnectionOpts, owner metav1.Object) (*mariadbv1alpha1.Connection, error) {
	objMeta :=
		metadata.NewMetadataBuilder(opts.Key).
			WithMetadata(opts.Metadata).
			Build()
	conn := &mariadbv1alpha1.Connection{
		ObjectMeta: objMeta,
		Spec: mariadbv1alpha1.ConnectionSpec{
			Username:             opts.Username,
			PasswordSecretKeyRef: opts.PasswordSecretKeyRef,
			Database:             opts.Database,
		},
	}
	if opts.MariaDB != nil {
		conn.Spec.MariaDBRef = &mariadbv1alpha1.MariaDBRef{
			ObjectReference: corev1.ObjectReference{
				Name: opts.MariaDB.Name,
			},
			WaitForIt: true,
		}
	} else if opts.MaxScale != nil {
		conn.Spec.MaxScaleRef = &corev1.ObjectReference{
			Name: opts.MaxScale.Name,
		}
	}
	if opts.Template != nil {
		conn.Spec.ConnectionTemplate = *opts.Template
	}
	if err := controllerutil.SetControllerReference(owner, conn, b.scheme); err != nil {
		return nil, fmt.Errorf("error setting controller reference to Connection: %v", err)
	}
	return conn, nil
}
