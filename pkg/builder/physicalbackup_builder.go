package builder

import (
	"fmt"
	"strings"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (b *Builder) BuildReplicaRecoveryPhysicalBackup(key types.NamespacedName, tpl *mariadbv1alpha1.PhysicalBackup,
	mariadb *mariadbv1alpha1.MariaDB) (*mariadbv1alpha1.PhysicalBackup, error) {
	physicalBackup := mariadbv1alpha1.PhysicalBackup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      key.Name,
			Namespace: key.Namespace,
		},
		Spec: *tpl.Spec.DeepCopy(),
	}
	rewriteReplicaRecoveryStoragePrefix(&physicalBackup.Spec, tpl.Spec.MariaDBRef.Name, mariadb.Name)
	physicalBackup.Spec.MariaDBRef = mariadbv1alpha1.MariaDBRef{
		ObjectReference: mariadbv1alpha1.ObjectReference{
			Name: mariadb.Name,
		},
		WaitForIt: false,
	}
	physicalBackup.Spec.Schedule = &mariadbv1alpha1.PhysicalBackupSchedule{
		Immediate: ptr.To(true),
	}
	if err := controllerutil.SetControllerReference(mariadb, &physicalBackup, b.scheme); err != nil {
		return nil, fmt.Errorf("error setting controller reference to PhysicalBackup: %v", err)
	}
	return &physicalBackup, nil
}

func rewriteReplicaRecoveryStoragePrefix(spec *mariadbv1alpha1.PhysicalBackupSpec, templateMariaDBName, mariadbName string) {
	if spec.Storage.S3 != nil {
		spec.Storage.S3.Prefix = rewriteReplicaRecoveryPrefix(spec.Storage.S3.Prefix, templateMariaDBName, mariadbName)
	}
	if spec.Storage.AzureBlob != nil {
		spec.Storage.AzureBlob.Prefix = rewriteReplicaRecoveryPrefix(spec.Storage.AzureBlob.Prefix, templateMariaDBName, mariadbName)
	}
}

func rewriteReplicaRecoveryPrefix(prefix, templateMariaDBName, mariadbName string) string {
	if prefix == "" || templateMariaDBName == "" || mariadbName == "" || templateMariaDBName == mariadbName {
		return prefix
	}
	trimmedPrefix := strings.TrimSuffix(prefix, "/")
	prefixParts := strings.Split(trimmedPrefix, "/")
	lastPartIndex := len(prefixParts) - 1
	if prefixParts[lastPartIndex] != templateMariaDBName {
		return prefix
	}
	prefixParts[lastPartIndex] = mariadbName
	rewrittenPrefix := strings.Join(prefixParts, "/")
	if strings.HasSuffix(prefix, "/") {
		return rewrittenPrefix + "/"
	}
	return rewrittenPrefix
}
