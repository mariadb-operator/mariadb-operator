package command

import (
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

var _ = Describe("ConnectionFlags", func() {
	DescribeTable("builds connection flags",
		func(opts *CommandOpts, mariadb *mariadbv1alpha1.MariaDB, flagOpts []ConnectionFlagOpt, want string, wantErr error) {
			got, err := ConnectionFlags(opts, mariadb, flagOpts...)
			if wantErr != nil {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal(wantErr.Error()))
				return
			}
			Expect(err).NotTo(HaveOccurred())
			Expect(got).To(Equal(want))
		},
		Entry("missing UserEnv",
			&CommandOpts{PasswordEnv: "PASS"},
			&mariadbv1alpha1.MariaDB{},
			nil,
			"",
			errors.New("UserEnv must be set"),
		),
		Entry("missing PasswordEnv",
			&CommandOpts{UserEnv: "USER"},
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "default",
				},
			},
			nil,
			"",
			errors.New("PasswordEnv must be set"),
		),
		Entry("basic flags with standalone",
			&CommandOpts{UserEnv: "USER", PasswordEnv: "PASS"},
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "default",
				},
				Spec: mariadbv1alpha1.MariaDBSpec{
					Port: 3306,
				},
			},
			nil,
			"--user=${USER} --password=${PASS} --host=test.default.svc.cluster.local --port=3306",
			nil,
		),
		Entry("basic flags with Galera",
			&CommandOpts{UserEnv: "USER", PasswordEnv: "PASS"},
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "default",
				},
				Spec: mariadbv1alpha1.MariaDBSpec{
					Port: 3306,
					Galera: &mariadbv1alpha1.Galera{
						Enabled: true,
					},
				},
			},
			nil,
			"--user=${USER} --password=${PASS} --host=test-primary.default.svc.cluster.local --port=3306",
			nil,
		),
		Entry("with database",
			&CommandOpts{UserEnv: "USER", PasswordEnv: "PASS", Database: ptr.To("test")},
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "default",
				},
				Spec: mariadbv1alpha1.MariaDBSpec{
					Port: 3306,
					Galera: &mariadbv1alpha1.Galera{
						Enabled: true,
					},
				},
			},
			nil,
			"--user=${USER} --password=${PASS} --host=test-primary.default.svc.cluster.local --port=3306 --database=test",
			nil,
		),
		Entry("with host override",
			&CommandOpts{UserEnv: "USER", PasswordEnv: "PASS"},
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "default",
				},
				Spec: mariadbv1alpha1.MariaDBSpec{
					Port: 3306,
					Galera: &mariadbv1alpha1.Galera{
						Enabled: true,
					},
				},
			},
			[]ConnectionFlagOpt{WithHostConnectionFlag("custom-host")},
			"--user=${USER} --password=${PASS} --host=custom-host --port=3306",
			nil,
		),
	)
})
