package builder

import (
	"fmt"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type ConnectionOpts struct {
	Key                  types.NamespacedName
	MariaDBRef           mariadbv1alpha1.MariaDBRef
	Username             string
	PasswordSecretKeyRef v1.SecretKeySelector
	Database             *string
	Template             *mariadbv1alpha1.ConnectionTemplate
}

func (b *Builder) BuildConnection(opts ConnectionOpts, owner metav1.Object) (*mariadbv1alpha1.Connection, error) {
	conn := &mariadbv1alpha1.Connection{
		ObjectMeta: metav1.ObjectMeta{
			Name:      opts.Key.Name,
			Namespace: opts.Key.Namespace,
		},
		Spec: mariadbv1alpha1.ConnectionSpec{
			MariaDBRef:           opts.MariaDBRef,
			Username:             opts.Username,
			PasswordSecretKeyRef: opts.PasswordSecretKeyRef,
			Database:             opts.Database,
		},
	}
	if opts.Template != nil {
		conn.Spec.ConnectionTemplate = *opts.Template
	}
	if err := controllerutil.SetControllerReference(owner, conn, b.scheme); err != nil {
		return nil, fmt.Errorf("error setting controller reference to Connection: %v", err)
	}
	return conn, nil
}
