package builder

import (
	"fmt"

	databasev1alpha1 "github.com/mmontes11/mariadb-operator/api/v1alpha1"
	labels "github.com/mmontes11/mariadb-operator/pkg/builder/labels"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (b *Builder) BuildRestoreMariaDb(mariaDb *databasev1alpha1.MariaDB, backupRef corev1.LocalObjectReference,
	key types.NamespacedName) (*databasev1alpha1.RestoreMariaDB, error) {
	restoreLabels :=
		labels.NewLabelsBuilder().
			WithApp(appMariaDb).
			WithInstance(mariaDb.Name).
			Build()
	restore := &databasev1alpha1.RestoreMariaDB{
		ObjectMeta: metav1.ObjectMeta{
			Name:      key.Name,
			Namespace: key.Namespace,
			Labels:    restoreLabels,
		},
		Spec: databasev1alpha1.RestoreMariaDBSpec{
			MariaDBRef: corev1.LocalObjectReference{
				Name: mariaDb.Name,
			},
			BackupRef:      backupRef,
			WaitForMariaDB: true,
		},
	}
	if err := controllerutil.SetControllerReference(mariaDb, restore, b.scheme); err != nil {
		return nil, fmt.Errorf("error setting controller reference to bootstrapping restore Job: %v", err)
	}
	return restore, nil
}
