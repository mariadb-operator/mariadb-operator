package builder

import (
	"testing"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestBuildReplicaRecoveryPhysicalBackupRewritesObjectStorePrefix(t *testing.T) {
	builder := newDefaultTestBuilder(t)
	tpl := &mariadbv1alpha1.PhysicalBackup{
		Spec: mariadbv1alpha1.PhysicalBackupSpec{
			MariaDBRef: mariadbv1alpha1.MariaDBRef{
				ObjectReference: mariadbv1alpha1.ObjectReference{
					Name: "db-cluster",
				},
			},
			Storage: mariadbv1alpha1.PhysicalBackupStorage{
				S3: &mariadbv1alpha1.S3{
					Prefix: "mariadb/yacodedev-internal/db-cluster",
				},
			},
		},
	}
	mariadb := &mariadbv1alpha1.MariaDB{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "testing-db-cluster",
			Namespace: "yacodedev-internal",
		},
	}

	backup, err := builder.BuildReplicaRecoveryPhysicalBackup(
		types.NamespacedName{Name: "testing-db-cluster-pb-recovery", Namespace: mariadb.Namespace},
		tpl,
		mariadb,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got, want := backup.Spec.Storage.S3.Prefix, "mariadb/yacodedev-internal/testing-db-cluster"; got != want {
		t.Fatalf("unexpected recovery S3 prefix, want %q got %q", want, got)
	}
	if got, want := tpl.Spec.Storage.S3.Prefix, "mariadb/yacodedev-internal/db-cluster"; got != want {
		t.Fatalf("template S3 prefix was mutated, want %q got %q", want, got)
	}
	if backup.Spec.MariaDBRef.Name != mariadb.Name {
		t.Fatalf("expected recovery backup to reference target MariaDB, got %q", backup.Spec.MariaDBRef.Name)
	}
	if backup.Spec.Schedule == nil || backup.Spec.Schedule.Immediate == nil || !*backup.Spec.Schedule.Immediate {
		t.Fatalf("expected recovery backup to be immediate")
	}
}

func TestBuildReplicaRecoveryPhysicalBackupPreservesSharedPrefix(t *testing.T) {
	builder := newDefaultTestBuilder(t)
	tpl := &mariadbv1alpha1.PhysicalBackup{
		Spec: mariadbv1alpha1.PhysicalBackupSpec{
			MariaDBRef: mariadbv1alpha1.MariaDBRef{
				ObjectReference: mariadbv1alpha1.ObjectReference{
					Name: "db-cluster",
				},
			},
			Storage: mariadbv1alpha1.PhysicalBackupStorage{
				S3: &mariadbv1alpha1.S3{
					Prefix: "mariadb/shared",
				},
			},
		},
	}
	mariadb := &mariadbv1alpha1.MariaDB{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "testing-db-cluster",
			Namespace: "yacodedev-internal",
		},
	}

	backup, err := builder.BuildReplicaRecoveryPhysicalBackup(
		types.NamespacedName{Name: "testing-db-cluster-pb-recovery", Namespace: mariadb.Namespace},
		tpl,
		mariadb,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got, want := backup.Spec.Storage.S3.Prefix, "mariadb/shared"; got != want {
		t.Fatalf("unexpected recovery S3 prefix, want %q got %q", want, got)
	}
}
