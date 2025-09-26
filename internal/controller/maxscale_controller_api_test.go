package controller

import (
	"strings"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v25/api/v1alpha1"
	builderpki "github.com/mariadb-operator/mariadb-operator/v25/pkg/builder/pki"
	"k8s.io/utils/ptr"

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
			want: "MASTER_SSL=1,MASTER_SSL_CA=" + builderpki.CACertPath +
				",MASTER_SSL_CERT=" + builderpki.ServerCertPath +
				",MASTER_SSL_KEY=" + builderpki.ServerKeyPath,
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
			want: "MASTER_CONNECT_RETRY=10",
		}),
		Entry("replication enabled (custom values)", test{
			mxs: &mariadbv1alpha1.MaxScale{},
			mdb: &mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Replication: &mariadbv1alpha1.Replication{
						Enabled: true,
						ReplicationSpec: mariadbv1alpha1.ReplicationSpec{
							Replica: &mariadbv1alpha1.ReplicaReplication{
								ConnectionRetries: ptr.To(5),
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
							Replica: &mariadbv1alpha1.ReplicaReplication{
								ConnectionRetries: ptr.To(5),
							},
						},
					},
				},
			},
			want: strings.Join([]string{
				"MASTER_CONNECT_RETRY=5",
				"MASTER_SSL=1",
				"MASTER_SSL_CA=" + builderpki.CACertPath,
				"MASTER_SSL_CERT=" + builderpki.ServerCertPath,
				"MASTER_SSL_KEY=" + builderpki.ServerKeyPath,
			}, ","),
		}),
	)
})
