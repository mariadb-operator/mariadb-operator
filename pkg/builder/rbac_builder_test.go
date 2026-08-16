package builder

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("ServiceAccountMeta", func() {
	builder := newDefaultTestBuilder()
	key := types.NamespacedName{
		Name: "sa",
	}

	DescribeTable("should build the ServiceAccount with the expected metadata",
		func(meta *mariadbv1alpha1.Metadata, wantMeta *mariadbv1alpha1.Metadata) {
			sa, err := builder.BuildServiceAccount(key, &mariadbv1alpha1.MariaDB{}, meta)
			Expect(err).NotTo(HaveOccurred())
			assertObjectMeta(&sa.ObjectMeta, wantMeta.Labels, wantMeta.Annotations)
		},
		Entry("no meta",
			nil,
			&mariadbv1alpha1.Metadata{
				Labels:      map[string]string{},
				Annotations: map[string]string{},
			},
		),
		Entry("meta",
			&mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"database.myorg.io": "mariadb",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
			&mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"database.myorg.io": "mariadb",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
		),
	)
})

var _ = Describe("RoleMeta", func() {
	builder := newDefaultTestBuilder()
	key := types.NamespacedName{
		Name: "role",
	}
	rules := []rbacv1.PolicyRule{}

	DescribeTable("should build the Role with the expected metadata",
		func(mariadb *mariadbv1alpha1.MariaDB, wantMeta *mariadbv1alpha1.Metadata) {
			role, err := builder.BuildRole(key, mariadb, mariadb.Spec.InheritMetadata, rules)
			Expect(err).NotTo(HaveOccurred())
			assertObjectMeta(&role.ObjectMeta, wantMeta.Labels, wantMeta.Annotations)
		},
		Entry("no meta",
			&mariadbv1alpha1.MariaDB{},
			&mariadbv1alpha1.Metadata{
				Labels:      map[string]string{},
				Annotations: map[string]string{},
			},
		),
		Entry("meta",
			&mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					InheritMetadata: &mariadbv1alpha1.Metadata{
						Labels: map[string]string{
							"database.myorg.io": "mariadb",
						},
						Annotations: map[string]string{
							"database.myorg.io": "mariadb",
						},
					},
				},
			},
			&mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"database.myorg.io": "mariadb",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
		),
	)
})

var _ = Describe("RoleBindingMeta", func() {
	builder := newDefaultTestBuilder()
	key := types.NamespacedName{
		Name: "rolebinding",
	}
	sa := corev1.ServiceAccount{}
	roleRef := rbacv1.RoleRef{}

	DescribeTable("should build the RoleBinding with the expected metadata",
		func(mariadb *mariadbv1alpha1.MariaDB, wantMeta *mariadbv1alpha1.Metadata) {
			role, err := builder.BuildRoleBinding(key, mariadb, mariadb.Spec.InheritMetadata, &sa, roleRef)
			Expect(err).NotTo(HaveOccurred())
			assertObjectMeta(&role.ObjectMeta, wantMeta.Labels, wantMeta.Annotations)
		},
		Entry("no meta",
			&mariadbv1alpha1.MariaDB{},
			&mariadbv1alpha1.Metadata{
				Labels:      map[string]string{},
				Annotations: map[string]string{},
			},
		),
		Entry("meta",
			&mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					InheritMetadata: &mariadbv1alpha1.Metadata{
						Labels: map[string]string{
							"database.myorg.io": "mariadb",
						},
						Annotations: map[string]string{
							"database.myorg.io": "mariadb",
						},
					},
				},
			},
			&mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"database.myorg.io": "mariadb",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
		),
	)
})

var _ = Describe("ClusterRoleBindingMeta", func() {
	builder := newDefaultTestBuilder()
	key := types.NamespacedName{
		Name: "clusterrolebinding",
	}
	sa := corev1.ServiceAccount{}
	roleRef := rbacv1.RoleRef{}

	DescribeTable("should build the ClusterRoleBinding with the expected metadata",
		func(mariadb *mariadbv1alpha1.MariaDB, wantMeta *mariadbv1alpha1.Metadata) {
			role, err := builder.BuildClusterRoleBinding(key, mariadb, mariadb.Spec.InheritMetadata, &sa, roleRef)
			Expect(err).NotTo(HaveOccurred())
			assertObjectMeta(&role.ObjectMeta, wantMeta.Labels, wantMeta.Annotations)
		},
		Entry("no meta",
			&mariadbv1alpha1.MariaDB{},
			&mariadbv1alpha1.Metadata{
				Labels:      map[string]string{},
				Annotations: map[string]string{},
			},
		),
		Entry("meta",
			&mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					InheritMetadata: &mariadbv1alpha1.Metadata{
						Labels: map[string]string{
							"database.myorg.io": "mariadb",
						},
						Annotations: map[string]string{
							"database.myorg.io": "mariadb",
						},
					},
				},
			},
			&mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"database.myorg.io": "mariadb",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
		),
	)
})
