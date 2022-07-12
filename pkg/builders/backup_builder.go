package builders

import (
	databasev1alpha1 "github.com/mmontes11/mariadb-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func BuildRestoreMariaDb(mariaDbRef corev1.LocalObjectReference, backupRef corev1.LocalObjectReference,
	key types.NamespacedName) *databasev1alpha1.RestoreMariaDB {
	return &databasev1alpha1.RestoreMariaDB{
		ObjectMeta: metav1.ObjectMeta{
			Name:      key.Name,
			Namespace: key.Namespace,
		},
		Spec: databasev1alpha1.RestoreMariaDBSpec{
			MariaDBRef: mariaDbRef,
			BackupRef:  backupRef,
		},
	}
}
