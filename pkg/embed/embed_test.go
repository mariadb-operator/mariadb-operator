package embed

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/environment"
)

var _ = Describe("ReadEntrypoint", func() {
	DescribeTable("reads entrypoint based on mariadb version",
		func(mariadb *mariadbv1alpha1.MariaDB, env *environment.OperatorEnv, wantBytes, wantErr bool) {
			bytes, err := ReadEntrypoint(context.Background(), mariadb, env)
			if wantBytes {
				Expect(bytes).NotTo(BeNil())
			} else {
				Expect(bytes).To(BeNil())
			}
			if wantErr {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
			}
		},
		Entry("empty", &mariadbv1alpha1.MariaDB{}, &environment.OperatorEnv{}, false, true),
		Entry("empty with default",
			&mariadbv1alpha1.MariaDB{},
			&environment.OperatorEnv{MariadbDefaultVersion: "10.11"},
			true, false,
		),
		Entry("invalid version",
			&mariadbv1alpha1.MariaDB{Spec: mariadbv1alpha1.MariaDBSpec{Image: "mariadb:foo"}},
			&environment.OperatorEnv{},
			false, true,
		),
		Entry("invalid version with default",
			&mariadbv1alpha1.MariaDB{Spec: mariadbv1alpha1.MariaDBSpec{Image: "mariadb:foo"}},
			&environment.OperatorEnv{MariadbDefaultVersion: "10.11"},
			true, false,
		),
		Entry("sha256",
			&mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Image: "mariadb@sha256:3f48454b6a33e094af6d23ced54645ec0533cb11854d07738920852ca48e390d",
				},
			},
			&environment.OperatorEnv{},
			false, true,
		),
		Entry("sha256 with default",
			&mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Image: "mariadb@sha256:3f48454b6a33e094af6d23ced54645ec0533cb11854d07738920852ca48e390d",
				},
			},
			&environment.OperatorEnv{MariadbDefaultVersion: "10.11"},
			true, false,
		),
		Entry("unsupported version",
			&mariadbv1alpha1.MariaDB{Spec: mariadbv1alpha1.MariaDBSpec{Image: "mariadb:8.0.0"}},
			&environment.OperatorEnv{},
			false, true,
		),
		Entry("unsupported version with default",
			&mariadbv1alpha1.MariaDB{Spec: mariadbv1alpha1.MariaDBSpec{Image: "mariadb:8.0.0"}},
			&environment.OperatorEnv{MariadbDefaultVersion: "10.11"},
			true, false,
		),
		Entry("supported version",
			&mariadbv1alpha1.MariaDB{Spec: mariadbv1alpha1.MariaDBSpec{Image: "mariadb:10.11.8"}},
			nil,
			true, false,
		),
		Entry("supported registry version",
			&mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Image: "registry-1.docker.io/v2/library/mariadb:10.6.18-14",
				},
			},
			nil,
			true, false,
		),
		Entry("invalid default",
			&mariadbv1alpha1.MariaDB{Spec: mariadbv1alpha1.MariaDBSpec{Image: ""}},
			&environment.OperatorEnv{MariadbDefaultVersion: "latest"},
			false, true,
		),
	)
})
