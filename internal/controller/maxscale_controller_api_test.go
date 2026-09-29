package controller

import (
	"strings"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	builderpki "github.com/mariadb-operator/mariadb-operator/v26/pkg/builder/pki"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/refresolver"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("MaxScale Replication options", Label("basic"), func() {
	type test struct {
		mxs  *mariadbv1alpha1.MaxScale
		mdb  *mariadbv1alpha1.MariaDB
		want string
	}

	DescribeTable("returns expected replication custom options",
		func(t test) {
			api := newMaxScaleAPI(t.mxs, nil, nil)
			Expect(api.maxScaleReplicationCustomOptions(t.mdb)).To(Equal(t.want))
		},
		Entry("no TLS (nil TLS)", test{
			mxs:  &mariadbv1alpha1.MaxScale{},
			want: "",
		}),
		Entry("TLS present but disabled", test{
			mxs: &mariadbv1alpha1.MaxScale{
				Spec: mariadbv1alpha1.MaxScaleSpec{
					TLS: &mariadbv1alpha1.MaxScaleTLS{
						Enabled: false,
					},
				},
			},
			want: "",
		}),
		Entry("TLS enabled but replication SSL disabled", test{
			mxs: &mariadbv1alpha1.MaxScale{
				Spec: mariadbv1alpha1.MaxScaleSpec{
					TLS: &mariadbv1alpha1.MaxScaleTLS{
						Enabled:               true,
						ReplicationSSLEnabled: ptr.To(false),
					},
				},
			},
			want: "",
		}),
		Entry("TLS enabled and replication SSL enabled", test{
			mxs: &mariadbv1alpha1.MaxScale{
				Spec: mariadbv1alpha1.MaxScaleSpec{
					TLS: &mariadbv1alpha1.MaxScaleTLS{
						Enabled:               true,
						ReplicationSSLEnabled: ptr.To(true),
					},
				},
			},
			want: strings.Join([]string{
				"MASTER_SSL=1",
				"MASTER_SSL_CA='" + builderpki.CACertPath + "'",
				"MASTER_SSL_CERT='" + builderpki.ClientCertPath + "'",
				"MASTER_SSL_KEY='" + builderpki.ClientKeyPath + "'",
				"MASTER_SSL_VERIFY_SERVER_CERT=1",
			}, ","),
		}),
		Entry("replication enabled (defaults)", test{
			mxs: &mariadbv1alpha1.MaxScale{},
			mdb: &mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Replication: &mariadbv1alpha1.Replication{
						Enabled: true,
					},
				},
			},
			// No MASTER_CONNECT_RETRY unless explicitly set.
			want: "",
		}),
		Entry("replication enabled (custom values)", test{
			mxs: &mariadbv1alpha1.MaxScale{},
			mdb: &mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Replication: &mariadbv1alpha1.Replication{
						Enabled: true,
						ReplicationSpec: mariadbv1alpha1.ReplicationSpec{
							Replica: mariadbv1alpha1.ReplicaReplication{
								ConnectionRetrySeconds: ptr.To(5),
							},
						},
					},
				},
			},
			want: "MASTER_CONNECT_RETRY=5",
		}),
		Entry("TLS+replication SSL enabled and MariaDB replication (custom)", test{
			mxs: &mariadbv1alpha1.MaxScale{
				Spec: mariadbv1alpha1.MaxScaleSpec{
					TLS: &mariadbv1alpha1.MaxScaleTLS{
						Enabled:               true,
						ReplicationSSLEnabled: ptr.To(true),
					},
				},
			},
			mdb: &mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Replication: &mariadbv1alpha1.Replication{
						Enabled: true,
						ReplicationSpec: mariadbv1alpha1.ReplicationSpec{
							Replica: mariadbv1alpha1.ReplicaReplication{
								ConnectionRetrySeconds: ptr.To(5),
							},
						},
					},
				},
			},
			want: strings.Join([]string{
				"MASTER_CONNECT_RETRY=5",
				"MASTER_SSL=1",
				"MASTER_SSL_CA='" + builderpki.CACertPath + "'",
				"MASTER_SSL_CERT='" + builderpki.ClientCertPath + "'",
				"MASTER_SSL_KEY='" + builderpki.ClientKeyPath + "'",
				"MASTER_SSL_VERIFY_SERVER_CERT=1",
			}, ","),
		}),
	)
})

var _ = Describe("MaxScale servers hash", Label("basic"), func() {
	It("should change when MariaDB-derived server attributes change", func() {
		scheme := runtime.NewScheme()
		Expect(mariadbv1alpha1.AddToScheme(scheme)).To(Succeed())

		mdb := &mariadbv1alpha1.MariaDB{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mariadb-servers-hash",
				Namespace: "default",
			},
			Spec: mariadbv1alpha1.MariaDBSpec{
				Replication: &mariadbv1alpha1.Replication{
					Enabled: true,
				},
			},
		}
		mxs := &mariadbv1alpha1.MaxScale{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "maxscale-servers-hash",
				Namespace: "default",
			},
			Spec: mariadbv1alpha1.MaxScaleSpec{
				MariaDBRef: &mariadbv1alpha1.MariaDBRef{
					ObjectReference: mariadbv1alpha1.ObjectReference{
						Name: mdb.Name,
					},
				},
				Servers: []mariadbv1alpha1.MaxScaleServer{
					{
						Name:    "mariadb-0",
						Address: "mariadb-0.mariadb-internal.default.svc.cluster.local",
						Port:    3306,
					},
				},
			},
		}
		req := &requestMaxScale{
			mxs: mxs,
		}

		hashWithMariaDB := func(mdb *mariadbv1alpha1.MariaDB) string {
			client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(mdb).Build()
			r := &MaxScaleReconciler{
				RefResolver: refresolver.New(client),
			}
			hash, err := r.serversHash(testCtx, req)
			Expect(err).ToNot(HaveOccurred())
			return hash
		}

		hash := hashWithMariaDB(mdb.DeepCopy())
		Expect(hashWithMariaDB(mdb.DeepCopy())).To(Equal(hash))

		mdbWithRetry := mdb.DeepCopy()
		mdbWithRetry.Spec.Replication.Replica.ConnectionRetrySeconds = ptr.To(5)
		Expect(hashWithMariaDB(mdbWithRetry)).NotTo(Equal(hash))
	})
})
