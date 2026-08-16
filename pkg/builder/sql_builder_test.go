package builder

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
)

var _ = Describe("UserMeta", func() {
	builder := newDefaultTestBuilder()
	key := types.NamespacedName{
		Name: "user",
	}
	DescribeTable("BuildUser meta",
		func(opts UserOpts, wantMeta *mariadbv1alpha1.Metadata) {
			user, err := builder.BuildUser(key, &mariadbv1alpha1.MariaDB{}, opts)
			Expect(err).NotTo(HaveOccurred())
			assertObjectMeta(&user.ObjectMeta, wantMeta.Labels, wantMeta.Annotations)
		},
		Entry("no meta",
			UserOpts{},
			&mariadbv1alpha1.Metadata{
				Labels:      map[string]string{},
				Annotations: map[string]string{},
			},
		),
		Entry("meta",
			UserOpts{
				Metadata: &mariadbv1alpha1.Metadata{
					Labels: map[string]string{
						"database.myorg.io": "mariadb",
					},
					Annotations: map[string]string{
						"database.myorg.io": "mariadb",
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

var _ = Describe("UserCleanupPolicy", func() {
	builder := newDefaultTestBuilder()
	key := types.NamespacedName{
		Name: "user",
	}
	DescribeTable("BuildUser cleanupPolicy",
		func(opts UserOpts, wantCleanupPolicy *mariadbv1alpha1.CleanupPolicy) {
			user, err := builder.BuildUser(key, &mariadbv1alpha1.MariaDB{}, opts)
			Expect(err).NotTo(HaveOccurred())
			Expect(user.Spec.CleanupPolicy).To(Equal(wantCleanupPolicy))
		},
		Entry("no cleanupPolicy",
			UserOpts{},
			nil,
		),
		Entry("cleanupPolicy",
			UserOpts{
				CleanupPolicy: ptr.To(mariadbv1alpha1.CleanupPolicySkip),
			},
			ptr.To(mariadbv1alpha1.CleanupPolicySkip),
		),
	)
})

var _ = Describe("GrantMeta", func() {
	builder := newDefaultTestBuilder()
	key := types.NamespacedName{
		Name: "grant",
	}
	DescribeTable("BuildGrant meta",
		func(opts GrantOpts, wantMeta *mariadbv1alpha1.Metadata) {
			grant, err := builder.BuildGrant(key, &mariadbv1alpha1.MariaDB{}, opts)
			Expect(err).NotTo(HaveOccurred())
			assertObjectMeta(&grant.ObjectMeta, wantMeta.Labels, wantMeta.Annotations)
		},
		Entry("no meta",
			GrantOpts{},
			&mariadbv1alpha1.Metadata{
				Labels:      map[string]string{},
				Annotations: map[string]string{},
			},
		),
		Entry("meta",
			GrantOpts{
				Metadata: &mariadbv1alpha1.Metadata{
					Labels: map[string]string{
						"database.myorg.io": "mariadb",
					},
					Annotations: map[string]string{
						"database.myorg.io": "mariadb",
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

var _ = Describe("GrantCleanupPolicy", func() {
	builder := newDefaultTestBuilder()
	key := types.NamespacedName{
		Name: "grant",
	}
	DescribeTable("BuildGrant cleanupPolicy",
		func(opts GrantOpts, wantCleanupPolicy *mariadbv1alpha1.CleanupPolicy) {
			grant, err := builder.BuildGrant(key, &mariadbv1alpha1.MariaDB{}, opts)
			Expect(err).NotTo(HaveOccurred())
			Expect(grant.Spec.CleanupPolicy).To(Equal(wantCleanupPolicy))
		},
		Entry("no cleanupPolicy",
			GrantOpts{},
			nil,
		),
		Entry("cleanupPolicy",
			GrantOpts{
				CleanupPolicy: ptr.To(mariadbv1alpha1.CleanupPolicySkip),
			},
			ptr.To(mariadbv1alpha1.CleanupPolicySkip),
		),
	)
})

var _ = Describe("DatabaseCleanupPolicy", func() {
	builder := newDefaultTestBuilder()
	key := types.NamespacedName{
		Name: "database",
	}
	DescribeTable("BuildDatabase cleanupPolicy",
		func(opts DatabaseOpts, wantCleanupPolicy *mariadbv1alpha1.CleanupPolicy) {
			database, err := builder.BuildDatabase(key, &mariadbv1alpha1.MariaDB{}, opts)
			Expect(err).NotTo(HaveOccurred())
			Expect(database.Spec.CleanupPolicy).To(Equal(wantCleanupPolicy))
		},
		Entry("no cleanupPolicy",
			DatabaseOpts{},
			nil,
		),
		Entry("cleanupPolicy",
			DatabaseOpts{
				CleanupPolicy: ptr.To(mariadbv1alpha1.CleanupPolicySkip),
			},
			ptr.To(mariadbv1alpha1.CleanupPolicySkip),
		),
	)
})
