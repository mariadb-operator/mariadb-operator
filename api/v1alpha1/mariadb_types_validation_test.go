package v1alpha1

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

var _ = Describe("MariaDB validation", func() {
	Context("When creating a MariaDB Resource", func() {
		meta := metav1.ObjectMeta{
			Name:      "mariadb-validation",
			Namespace: testNamespace,
		}
		DescribeTable(
			"Should validate",
			func(mdb *MariaDB, wantErr bool, validationMessage string) {
				_ = k8sClient.Delete(testCtx, mdb)
				err := k8sClient.Create(testCtx, mdb)
				if wantErr {
					Expect(err).To(HaveOccurred(), "Expected there to be a validation error, but there was none")
					Expect(err.Error()).To(Equal(validationMessage))
				} else {
					Expect(err).ToNot(HaveOccurred(), "Did not expect there to be a validation error, but there was one.")
				}
			},

			Entry(
				"Valid replicas when not even",
				&MariaDB{
					ObjectMeta: meta,
					Spec: MariaDBSpec{
						Replicas: 3,
						Storage: Storage{
							Size: ptr.To(resource.MustParse("100Mi")),
						},
					},
				},
				false,
				"",
			),
			Entry(
				"Valid replicas when even",
				&MariaDB{
					ObjectMeta: meta,
					Spec: MariaDBSpec{
						Replicas: 2,
						Storage: Storage{
							Size: ptr.To(resource.MustParse("100Mi")),
						},
					},
				},
				false,
				"",
			),
			Entry(
				"Valid replicas with replication when not even",
				&MariaDB{
					ObjectMeta: meta,
					Spec: MariaDBSpec{
						Replicas: 3,
						Storage: Storage{
							Size: ptr.To(resource.MustParse("100Mi")),
						},
						Replication: &Replication{
							Enabled: true,
						},
					},
				},
				false,
				"",
			),
			Entry(
				"Valid replicas with replication when even",
				&MariaDB{
					ObjectMeta: meta,
					Spec: MariaDBSpec{
						Replicas: 2,
						Storage: Storage{
							Size: ptr.To(resource.MustParse("100Mi")),
						},
						Replication: &Replication{
							Enabled: true,
						},
					},
				},
				false,
				"",
			),
			Entry(
				"Valid Galera replicas",
				&MariaDB{
					ObjectMeta: meta,
					Spec: MariaDBSpec{
						Galera: &Galera{
							Enabled: true,
							GaleraSpec: GaleraSpec{
								SST:            SSTMariaBackup,
								ReplicaThreads: 1,
							},
						},
						Replicas: 3,
						Storage: Storage{
							Size: ptr.To(resource.MustParse("100Mi")),
						},
					},
				},
				false,
				"",
			),
			Entry(
				"Valid Galera replicas when even and replicasAllowEvenNumber is set",
				&MariaDB{
					ObjectMeta: meta,
					Spec: MariaDBSpec{
						Galera: &Galera{
							Enabled: true,
							GaleraSpec: GaleraSpec{
								SST:            SSTMariaBackup,
								ReplicaThreads: 1,
							},
						},
						Replicas:                2,
						ReplicasAllowEvenNumber: true,
						Storage: Storage{
							Size: ptr.To(resource.MustParse("100Mi")),
						},
					},
				},
				false,
				"",
			),
			Entry(
				"Invalid Galera replicas",
				&MariaDB{
					ObjectMeta: meta,
					Spec: MariaDBSpec{
						Galera: &Galera{
							Enabled: true,
							GaleraSpec: GaleraSpec{
								SST:            SSTMariaBackup,
								ReplicaThreads: 1,
							},
						},
						Replicas: 2,
						Storage: Storage{
							Size: ptr.To(resource.MustParse("100Mi")),
						},
					},
				},
				true,
				"MariaDB.k8s.mariadb.com \"mariadb-validation\" is invalid: spec: Invalid value: \"object\": An odd number of MariaDB instances (mariadb.spec.replicas) is required to avoid split brain situations for Galera. Use 'mariadb.spec.replicasAllowEvenNumber: true' to disable this validation.", //nolint
			),
		)
	})
})
