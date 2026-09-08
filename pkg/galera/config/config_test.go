package config

import (
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/environment"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

var _ = Describe("Galera config marshal", func() {
	DescribeTable("marshaling a Galera config",
		func(mariadb *mariadbv1alpha1.MariaDB, podEnv *environment.PodEnvironment, wantConfig string, wantErr bool) {
			bytes, err := NewConfigFile(mariadb, logr.Discard()).Marshal(podEnv)

			if wantErr {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
			}
			Expect(string(bytes)).To(Equal(wantConfig))
		},
		Entry(
			"no replicas",
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: v1.ObjectMeta{
					Name:      "mariadb-galera",
					Namespace: "default",
				},
				Spec: mariadbv1alpha1.MariaDBSpec{
					Galera: &mariadbv1alpha1.Galera{
						Enabled: true,
						GaleraSpec: mariadbv1alpha1.GaleraSpec{
							SST:            mariadbv1alpha1.SSTRsync,
							ReplicaThreads: 1,
						},
					},
					Replicas: 0,
				},
			},
			&environment.PodEnvironment{
				PodName:             "mariadb-galera-0",
				PodIP:               "10.244.0.32",
				MariadbRootPassword: "mariadb",
			},
			"",
			true,
		),
		Entry(
			"multicluster all params",
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: v1.ObjectMeta{
					Name:      "mariadb-galera",
					Namespace: "default",
				},
				Spec: mariadbv1alpha1.MariaDBSpec{
					Galera: &mariadbv1alpha1.Galera{
						Enabled: true,
						GaleraSpec: mariadbv1alpha1.GaleraSpec{
							SST:            mariadbv1alpha1.SSTMariaBackup,
							GaleraLibPath:  "/usr/lib/galera/libgalera_smm.so",
							ReplicaThreads: 1,
							GtidDomainID:   ptr.To(0),
							ServerID:       ptr.To(100),
						},
					},
					MultiCluster: &mariadbv1alpha1.MultiCluster{
						Enabled: true,
					},
					Replicas: 3,
				},
			},
			&environment.PodEnvironment{
				PodName:             "mariadb-galera-1",
				PodIP:               "10.244.0.32",
				MariadbRootPassword: "mariadb",
			},
			//nolint:lll
			`[mariadb]
bind_address=*
default_storage_engine=InnoDB
binlog_format=row
innodb_autoinc_lock_mode=2

# Cluster
wsrep_on=ON
wsrep_cluster_address="gcomm://mariadb-galera-0.mariadb-galera-internal.default.svc.cluster.local,mariadb-galera-1.mariadb-galera-internal.default.svc.cluster.local,mariadb-galera-2.mariadb-galera-internal.default.svc.cluster.local"
wsrep_cluster_name=mariadb-operator
wsrep_slave_threads=1

# Node
wsrep_node_address="10.244.0.32"
wsrep_node_name="mariadb-galera-1"

# Provider
wsrep_provider=/usr/lib/galera/libgalera_smm.so
wsrep_provider_options="gmcast.listen_addr=tcp://0.0.0.0:4567;ist.recv_addr=10.244.0.32:4568;socket.ssl=false"

# SST
wsrep_sst_method="mariabackup"
wsrep_sst_auth="root:mariadb"
wsrep_sst_receive_address="10.244.0.32:4444"

# Multi-cluster
log-bin
log_slave_updates=ON
wsrep_gtid_mode=ON
wsrep_gtid_domain_id=0
gtid_domain_id=2
server_id=100
`,
			false,
		),
		Entry(
			"multicluster invalid Pod",
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: v1.ObjectMeta{
					Name:      "mariadb-galera",
					Namespace: "default",
				},
				Spec: mariadbv1alpha1.MariaDBSpec{
					Galera: &mariadbv1alpha1.Galera{
						Enabled: true,
						GaleraSpec: mariadbv1alpha1.GaleraSpec{
							SST:            mariadbv1alpha1.SSTMariaBackup,
							GaleraLibPath:  "/usr/lib/galera/libgalera_smm.so",
							ReplicaThreads: 1,
							GtidDomainID:   ptr.To(0),
							ServerID:       ptr.To(100),
						},
					},
					MultiCluster: &mariadbv1alpha1.MultiCluster{
						Enabled: true,
					},
					Replicas: 3,
				},
			},
			&environment.PodEnvironment{
				PodName:             "test",
				PodIP:               "10.244.0.32",
				MariadbRootPassword: "mariadb",
			},
			"",
			true,
		),
		Entry(
			"multicluster missing params",
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: v1.ObjectMeta{
					Name:      "mariadb-galera",
					Namespace: "default",
				},
				Spec: mariadbv1alpha1.MariaDBSpec{
					Galera: &mariadbv1alpha1.Galera{
						Enabled: true,
						GaleraSpec: mariadbv1alpha1.GaleraSpec{
							SST:            mariadbv1alpha1.SSTMariaBackup,
							GaleraLibPath:  "/usr/lib/galera/libgalera_smm.so",
							ReplicaThreads: 1,
							// Intentionally leaving GtidDomainID and ServerID nil
						},
					},
					MultiCluster: &mariadbv1alpha1.MultiCluster{
						Enabled: true,
					},
					Replicas: 3,
				},
			},
			&environment.PodEnvironment{
				PodName:             "mariadb-galera-1",
				PodIP:               "10.244.0.32",
				MariadbRootPassword: "mariadb",
			},
			//nolint:lll
			`[mariadb]
bind_address=*
default_storage_engine=InnoDB
binlog_format=row
innodb_autoinc_lock_mode=2

# Cluster
wsrep_on=ON
wsrep_cluster_address="gcomm://mariadb-galera-0.mariadb-galera-internal.default.svc.cluster.local,mariadb-galera-1.mariadb-galera-internal.default.svc.cluster.local,mariadb-galera-2.mariadb-galera-internal.default.svc.cluster.local"
wsrep_cluster_name=mariadb-operator
wsrep_slave_threads=1

# Node
wsrep_node_address="10.244.0.32"
wsrep_node_name="mariadb-galera-1"

# Provider
wsrep_provider=/usr/lib/galera/libgalera_smm.so
wsrep_provider_options="gmcast.listen_addr=tcp://0.0.0.0:4567;ist.recv_addr=10.244.0.32:4568;socket.ssl=false"

# SST
wsrep_sst_method="mariabackup"
wsrep_sst_auth="root:mariadb"
wsrep_sst_receive_address="10.244.0.32:4444"
`,
			false,
		),
		Entry(
			"Galera not enabled",
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: v1.ObjectMeta{
					Name:      "mariadb-galera",
					Namespace: "default",
				},
				Spec: mariadbv1alpha1.MariaDBSpec{
					Galera: &mariadbv1alpha1.Galera{
						Enabled: false,
					},
					Replicas: 0,
				},
			},
			&environment.PodEnvironment{
				PodName:             "mariadb-galera-0",
				PodIP:               "10.244.0.32",
				MariadbRootPassword: "mariadb",
			},
			"",
			true,
		),
		Entry(
			"invalid IP",
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: v1.ObjectMeta{
					Name:      "mariadb-galera",
					Namespace: "default",
				},
				Spec: mariadbv1alpha1.MariaDBSpec{
					Galera: &mariadbv1alpha1.Galera{
						Enabled: true,
					},
					Replicas: 0,
				},
			},
			&environment.PodEnvironment{
				PodName:             "mariadb-galera-0",
				PodIP:               "foo",
				MariadbRootPassword: "mariadb",
			},
			"",
			true,
		),
		Entry(
			"rsync",
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: v1.ObjectMeta{
					Name:      "mariadb-galera",
					Namespace: "default",
				},
				Spec: mariadbv1alpha1.MariaDBSpec{
					Galera: &mariadbv1alpha1.Galera{
						Enabled: true,
						GaleraSpec: mariadbv1alpha1.GaleraSpec{
							SST:            mariadbv1alpha1.SSTRsync,
							GaleraLibPath:  "/usr/lib/galera/libgalera_smm.so",
							ReplicaThreads: 1,
						},
					},
					Replicas: 3,
				},
			},
			&environment.PodEnvironment{
				PodName:             "mariadb-galera-0",
				PodIP:               "10.244.0.32",
				MariadbRootPassword: "mariadb",
			},
			//nolint:lll
			`[mariadb]
bind_address=*
default_storage_engine=InnoDB
binlog_format=row
innodb_autoinc_lock_mode=2

# Cluster
wsrep_on=ON
wsrep_cluster_address="gcomm://mariadb-galera-0.mariadb-galera-internal.default.svc.cluster.local,mariadb-galera-1.mariadb-galera-internal.default.svc.cluster.local,mariadb-galera-2.mariadb-galera-internal.default.svc.cluster.local"
wsrep_cluster_name=mariadb-operator
wsrep_slave_threads=1

# Node
wsrep_node_address="10.244.0.32"
wsrep_node_name="mariadb-galera-0"

# Provider
wsrep_provider=/usr/lib/galera/libgalera_smm.so
wsrep_provider_options="gmcast.listen_addr=tcp://0.0.0.0:4567;ist.recv_addr=10.244.0.32:4568;socket.ssl=false"

# SST
wsrep_sst_method="rsync"
wsrep_sst_receive_address="10.244.0.32:4444"
`,
			false,
		),
		Entry(
			"mariabackup",
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: v1.ObjectMeta{
					Name:      "mariadb-galera",
					Namespace: "default",
				},
				Spec: mariadbv1alpha1.MariaDBSpec{
					Galera: &mariadbv1alpha1.Galera{
						Enabled: true,
						GaleraSpec: mariadbv1alpha1.GaleraSpec{
							SST:            mariadbv1alpha1.SSTMariaBackup,
							GaleraLibPath:  "/usr/lib/galera/libgalera_smm.so",
							ReplicaThreads: 2,
						},
					},
					Replicas: 3,
				},
			},
			&environment.PodEnvironment{
				PodName:             "mariadb-galera-1",
				PodIP:               "10.244.0.32",
				MariadbRootPassword: "mariadb",
			},
			//nolint:lll
			`[mariadb]
bind_address=*
default_storage_engine=InnoDB
binlog_format=row
innodb_autoinc_lock_mode=2

# Cluster
wsrep_on=ON
wsrep_cluster_address="gcomm://mariadb-galera-0.mariadb-galera-internal.default.svc.cluster.local,mariadb-galera-1.mariadb-galera-internal.default.svc.cluster.local,mariadb-galera-2.mariadb-galera-internal.default.svc.cluster.local"
wsrep_cluster_name=mariadb-operator
wsrep_slave_threads=2

# Node
wsrep_node_address="10.244.0.32"
wsrep_node_name="mariadb-galera-1"

# Provider
wsrep_provider=/usr/lib/galera/libgalera_smm.so
wsrep_provider_options="gmcast.listen_addr=tcp://0.0.0.0:4567;ist.recv_addr=10.244.0.32:4568;socket.ssl=false"

# SST
wsrep_sst_method="mariabackup"
wsrep_sst_auth="root:mariadb"
wsrep_sst_receive_address="10.244.0.32:4444"
`,
			false,
		),
		Entry(
			"IPv6",
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: v1.ObjectMeta{
					Name:      "mariadb-galera",
					Namespace: "default",
				},
				Spec: mariadbv1alpha1.MariaDBSpec{
					Galera: &mariadbv1alpha1.Galera{
						Enabled: true,
						GaleraSpec: mariadbv1alpha1.GaleraSpec{
							SST:            mariadbv1alpha1.SSTMariaBackup,
							GaleraLibPath:  "/usr/lib/galera/libgalera_smm.so",
							ReplicaThreads: 1,
						},
					},
					Replicas: 3,
				},
			},
			&environment.PodEnvironment{
				PodName:             "mariadb-galera-1",
				PodIP:               "2001:db8::a1",
				MariadbRootPassword: "mariadb",
			},
			//nolint:lll
			`[mariadb]
bind_address=*
default_storage_engine=InnoDB
binlog_format=row
innodb_autoinc_lock_mode=2

# Cluster
wsrep_on=ON
wsrep_cluster_address="gcomm://mariadb-galera-0.mariadb-galera-internal.default.svc.cluster.local,mariadb-galera-1.mariadb-galera-internal.default.svc.cluster.local,mariadb-galera-2.mariadb-galera-internal.default.svc.cluster.local"
wsrep_cluster_name=mariadb-operator
wsrep_slave_threads=1

# Node
wsrep_node_address="2001:db8::a1"
wsrep_node_name="mariadb-galera-1"

# Provider
wsrep_provider=/usr/lib/galera/libgalera_smm.so
wsrep_provider_options="gmcast.listen_addr=tcp://[::]:4567;ist.recv_addr=[2001:db8::a1]:4568;socket.ssl=false"

# SST
wsrep_sst_method="mariabackup"
wsrep_sst_auth="root:mariadb"
wsrep_sst_receive_address="[2001:db8::a1]:4444"
`,
			false,
		),
		Entry(
			"Additional WSREP provider options",
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: v1.ObjectMeta{
					Name:      "mariadb-galera",
					Namespace: "default",
				},
				Spec: mariadbv1alpha1.MariaDBSpec{
					Galera: &mariadbv1alpha1.Galera{
						Enabled: true,
						GaleraSpec: mariadbv1alpha1.GaleraSpec{
							SST:            mariadbv1alpha1.SSTMariaBackup,
							GaleraLibPath:  "/usr/lib/galera/libgalera_smm.so",
							ReplicaThreads: 1,
							ProviderOptions: map[string]string{
								"gcache.size":  "1G",
								"gcs.fc_limit": "128",
							},
						},
					},
					Replicas: 3,
				},
			},
			&environment.PodEnvironment{
				PodName:             "mariadb-galera-1",
				PodIP:               "2001:db8::a1",
				MariadbRootPassword: "mariadb",
			},
			//nolint:lll
			`[mariadb]
bind_address=*
default_storage_engine=InnoDB
binlog_format=row
innodb_autoinc_lock_mode=2

# Cluster
wsrep_on=ON
wsrep_cluster_address="gcomm://mariadb-galera-0.mariadb-galera-internal.default.svc.cluster.local,mariadb-galera-1.mariadb-galera-internal.default.svc.cluster.local,mariadb-galera-2.mariadb-galera-internal.default.svc.cluster.local"
wsrep_cluster_name=mariadb-operator
wsrep_slave_threads=1

# Node
wsrep_node_address="2001:db8::a1"
wsrep_node_name="mariadb-galera-1"

# Provider
wsrep_provider=/usr/lib/galera/libgalera_smm.so
wsrep_provider_options="gcache.size=1G;gcs.fc_limit=128;gmcast.listen_addr=tcp://[::]:4567;ist.recv_addr=[2001:db8::a1]:4568;socket.ssl=false"

# SST
wsrep_sst_method="mariabackup"
wsrep_sst_auth="root:mariadb"
wsrep_sst_receive_address="[2001:db8::a1]:4444"
`,
			false,
		),
		Entry(
			"TLS",
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: v1.ObjectMeta{
					Name:      "mariadb-galera",
					Namespace: "default",
				},
				Spec: mariadbv1alpha1.MariaDBSpec{
					Image: "mariadb:10.11.8",
					Galera: &mariadbv1alpha1.Galera{
						Enabled: true,
						GaleraSpec: mariadbv1alpha1.GaleraSpec{
							SST:            mariadbv1alpha1.SSTMariaBackup,
							GaleraLibPath:  "/usr/lib/galera/libgalera_smm.so",
							ReplicaThreads: 2,
						},
					},
					TLS: &mariadbv1alpha1.TLS{
						Enabled:          true,
						Required:         ptr.To(true),
						GaleraSSTEnabled: ptr.To(true),
					},
					Replicas: 3,
				},
			},
			&environment.PodEnvironment{
				PodName:             "mariadb-galera-1",
				PodIP:               "10.244.0.32",
				MariadbRootPassword: "mariadb",
				TLSEnabled:          "true",
				TLSCACertPath:       "/etc/pki/ca.crt",
				TLSServerCertPath:   "/etc/pki/server.crt",
				TLSServerKeyPath:    "/etc/pki/server.key",
				TLSClientCertPath:   "/etc/pki/client.crt",
				TLSClientKeyPath:    "/etc/pki/client.key",
			},
			//nolint:lll
			`[mariadb]
bind_address=*
default_storage_engine=InnoDB
binlog_format=row
innodb_autoinc_lock_mode=2

# Cluster
wsrep_on=ON
wsrep_cluster_address="gcomm://mariadb-galera-0.mariadb-galera-internal.default.svc.cluster.local,mariadb-galera-1.mariadb-galera-internal.default.svc.cluster.local,mariadb-galera-2.mariadb-galera-internal.default.svc.cluster.local"
wsrep_cluster_name=mariadb-operator
wsrep_slave_threads=2

# Node
wsrep_node_address="10.244.0.32"
wsrep_node_name="mariadb-galera-1"

# Provider
wsrep_provider=/usr/lib/galera/libgalera_smm.so
wsrep_provider_options="gmcast.listen_addr=tcp://0.0.0.0:4567;ist.recv_addr=10.244.0.32:4568;socket.dynamic=false;socket.ssl=true;socket.ssl_ca=/etc/pki/ca.crt;socket.ssl_cert=/etc/pki/server.crt;socket.ssl_key=/etc/pki/server.key"

# SST
wsrep_sst_method="mariabackup"
wsrep_sst_auth="root:mariadb"
wsrep_sst_receive_address="10.244.0.32:4444"
[sst]
encrypt=3
tca=/etc/pki/ca.crt
tcert=/etc/pki/client.crt
tkey=/etc/pki/client.key
`,
			false,
		),
		Entry(
			"TLS with Galera SST disabled",
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: v1.ObjectMeta{
					Name:      "mariadb-galera",
					Namespace: "default",
				},
				Spec: mariadbv1alpha1.MariaDBSpec{
					Image: "mariadb:10.11.8",
					Galera: &mariadbv1alpha1.Galera{
						Enabled: true,
						GaleraSpec: mariadbv1alpha1.GaleraSpec{
							SST:            mariadbv1alpha1.SSTMariaBackup,
							GaleraLibPath:  "/usr/lib/galera/libgalera_smm.so",
							ReplicaThreads: 2,
						},
					},
					TLS: &mariadbv1alpha1.TLS{
						Enabled:          true,
						Required:         ptr.To(true),
						GaleraSSTEnabled: ptr.To(false),
					},
					Replicas: 3,
				},
			},
			&environment.PodEnvironment{
				PodName:             "mariadb-galera-1",
				PodIP:               "10.244.0.32",
				MariadbRootPassword: "mariadb",
				TLSEnabled:          "true",
				TLSCACertPath:       "/etc/pki/ca.crt",
				TLSServerCertPath:   "/etc/pki/server.crt",
				TLSServerKeyPath:    "/etc/pki/server.key",
				TLSClientCertPath:   "/etc/pki/client.crt",
				TLSClientKeyPath:    "/etc/pki/client.key",
			},
			//nolint:lll
			`[mariadb]
bind_address=*
default_storage_engine=InnoDB
binlog_format=row
innodb_autoinc_lock_mode=2

# Cluster
wsrep_on=ON
wsrep_cluster_address="gcomm://mariadb-galera-0.mariadb-galera-internal.default.svc.cluster.local,mariadb-galera-1.mariadb-galera-internal.default.svc.cluster.local,mariadb-galera-2.mariadb-galera-internal.default.svc.cluster.local"
wsrep_cluster_name=mariadb-operator
wsrep_slave_threads=2

# Node
wsrep_node_address="10.244.0.32"
wsrep_node_name="mariadb-galera-1"

# Provider
wsrep_provider=/usr/lib/galera/libgalera_smm.so
wsrep_provider_options="gmcast.listen_addr=tcp://0.0.0.0:4567;ist.recv_addr=10.244.0.32:4568;socket.dynamic=false;socket.ssl=true;socket.ssl_ca=/etc/pki/ca.crt;socket.ssl_cert=/etc/pki/server.crt;socket.ssl_key=/etc/pki/server.key"

# SST
wsrep_sst_method="mariabackup"
wsrep_sst_auth="root:mariadb"
wsrep_sst_receive_address="10.244.0.32:4444"
`,
			false,
		),
		Entry(
			"TLS with required disabled",
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: v1.ObjectMeta{
					Name:      "mariadb-galera",
					Namespace: "default",
				},
				Spec: mariadbv1alpha1.MariaDBSpec{
					Image: "mariadb:10.11.8",
					Galera: &mariadbv1alpha1.Galera{
						Enabled: true,
						GaleraSpec: mariadbv1alpha1.GaleraSpec{
							SST:            mariadbv1alpha1.SSTMariaBackup,
							GaleraLibPath:  "/usr/lib/galera/libgalera_smm.so",
							ReplicaThreads: 2,
						},
					},
					TLS: &mariadbv1alpha1.TLS{
						Enabled:          true,
						Required:         ptr.To(false),
						GaleraSSTEnabled: ptr.To(true),
					},
					Replicas: 3,
				},
			},
			&environment.PodEnvironment{
				PodName:             "mariadb-galera-1",
				PodIP:               "10.244.0.32",
				MariadbRootPassword: "mariadb",
				TLSEnabled:          "true",
				TLSCACertPath:       "/etc/pki/ca.crt",
				TLSServerCertPath:   "/etc/pki/server.crt",
				TLSServerKeyPath:    "/etc/pki/server.key",
				TLSClientCertPath:   "/etc/pki/client.crt",
				TLSClientKeyPath:    "/etc/pki/client.key",
			},
			//nolint:lll
			`[mariadb]
bind_address=*
default_storage_engine=InnoDB
binlog_format=row
innodb_autoinc_lock_mode=2

# Cluster
wsrep_on=ON
wsrep_cluster_address="gcomm://mariadb-galera-0.mariadb-galera-internal.default.svc.cluster.local,mariadb-galera-1.mariadb-galera-internal.default.svc.cluster.local,mariadb-galera-2.mariadb-galera-internal.default.svc.cluster.local"
wsrep_cluster_name=mariadb-operator
wsrep_slave_threads=2

# Node
wsrep_node_address="10.244.0.32"
wsrep_node_name="mariadb-galera-1"

# Provider
wsrep_provider=/usr/lib/galera/libgalera_smm.so
wsrep_provider_options="gmcast.listen_addr=tcp://0.0.0.0:4567;ist.recv_addr=10.244.0.32:4568;socket.dynamic=true;socket.ssl=true;socket.ssl_ca=/etc/pki/ca.crt;socket.ssl_cert=/etc/pki/server.crt;socket.ssl_key=/etc/pki/server.key"

# SST
wsrep_sst_method="mariabackup"
wsrep_sst_auth="root:mariadb"
wsrep_sst_receive_address="10.244.0.32:4444"
[sst]
encrypt=3
tca=/etc/pki/ca.crt
tcert=/etc/pki/client.crt
tkey=/etc/pki/client.key
`,
			false,
		),
	)
})

var _ = Describe("Galera config update", func() {
	DescribeTable("updating a Galera config",
		func(config string, podEnv *environment.PodEnvironment, wantConfig []byte, wantErr bool) {
			bytes, err := UpdateConfig([]byte(config), podEnv)
			if wantErr {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
			}
			Expect(string(bytes)).To(Equal(string(wantConfig)))
		},
		Entry(
			"invalid IP",
			`[mariadb]
bind_address=*
default_storage_engine=InnoDB
binlog_format=row
innodb_autoinc_lock_mode=2

# Cluster
wsrep_on=ON
wsrep_cluster_address="gcomm://mariadb-galera-0.mariadb-galera-internal.default.svc.cluster.local,mariadb-galera-1.mariadb-galera-internal.default.svc.cluster.local,mariadb-galera-2.mariadb-galera-internal.default.svc.cluster.local"
wsrep_cluster_name=mariadb-operator
wsrep_slave_threads=2

# Node
wsrep_node_address="10.244.0.32"
wsrep_node_name="mariadb-galera-1"

# Provider
wsrep_provider=/usr/lib/galera/libgalera_smm.so
wsrep_provider_options="gmcast.listen_addr=tcp://0.0.0.0:4567;ist.recv_addr=10.244.0.32:4568"

# SST
wsrep_sst_method="mariabackup"
wsrep_sst_auth="root:mariadb"
wsrep_sst_receive_address="10.244.0.32:4444"`,
			&environment.PodEnvironment{
				PodIP: "foo",
			},
			nil,
			true,
		),
		Entry(
			"IPv4",
			`[mariadb]
bind_address=*
default_storage_engine=InnoDB
binlog_format=row
innodb_autoinc_lock_mode=2

# Cluster
wsrep_on=ON
wsrep_cluster_address="gcomm://mariadb-galera-0.mariadb-galera-internal.default.svc.cluster.local,mariadb-galera-1.mariadb-galera-internal.default.svc.cluster.local,mariadb-galera-2.mariadb-galera-internal.default.svc.cluster.local"
wsrep_cluster_name=mariadb-operator
wsrep_slave_threads=2

# Node
wsrep_node_address="10.244.0.32"
wsrep_node_name="mariadb-galera-1"

# Provider
wsrep_provider=/usr/lib/galera/libgalera_smm.so
wsrep_provider_options="gmcast.listen_addr=tcp://0.0.0.0:4567;ist.recv_addr=10.244.0.32:4568"

# SST
wsrep_sst_method="mariabackup"
wsrep_sst_auth="root:mariadb"
wsrep_sst_receive_address="10.244.0.32:4444"`,
			&environment.PodEnvironment{
				PodIP: "10.244.0.33",
			},
			[]byte(`[mariadb]
bind_address=*
default_storage_engine=InnoDB
binlog_format=row
innodb_autoinc_lock_mode=2

# Cluster
wsrep_on=ON
wsrep_cluster_address="gcomm://mariadb-galera-0.mariadb-galera-internal.default.svc.cluster.local,mariadb-galera-1.mariadb-galera-internal.default.svc.cluster.local,mariadb-galera-2.mariadb-galera-internal.default.svc.cluster.local"
wsrep_cluster_name=mariadb-operator
wsrep_slave_threads=2

# Node
wsrep_node_address="10.244.0.33"
wsrep_node_name="mariadb-galera-1"

# Provider
wsrep_provider=/usr/lib/galera/libgalera_smm.so
wsrep_provider_options="gmcast.listen_addr=tcp://0.0.0.0:4567;ist.recv_addr=10.244.0.33:4568"

# SST
wsrep_sst_method="mariabackup"
wsrep_sst_auth="root:mariadb"
wsrep_sst_receive_address="10.244.0.33:4444"`),
			false,
		),
		Entry(
			"IPv6",
			`[mariadb]
bind_address=*
default_storage_engine=InnoDB
binlog_format=row
innodb_autoinc_lock_mode=2

# Cluster
wsrep_on=ON
wsrep_cluster_address="gcomm://mariadb-galera-0.mariadb-galera-internal.default.svc.cluster.local,mariadb-galera-1.mariadb-galera-internal.default.svc.cluster.local,mariadb-galera-2.mariadb-galera-internal.default.svc.cluster.local"
wsrep_cluster_name=mariadb-operator
wsrep_slave_threads=1

# Node
wsrep_node_address="2001:db8::a1"
wsrep_node_name="mariadb-galera-1"

# Provider
wsrep_provider=/usr/lib/galera/libgalera_smm.so
wsrep_provider_options="gcache.size=1G;gcs.fc_limit=128;gmcast.listen_addr=tcp://[::]:4567;ist.recv_addr=[2001:db8::a1]:4568"

# SST
wsrep_sst_method="mariabackup"
wsrep_sst_auth="root:mariadb"
wsrep_sst_receive_address="[2001:db8::a1]:4444"`,
			&environment.PodEnvironment{
				PodIP: "2001:db8::a2",
			},
			[]byte(`[mariadb]
bind_address=*
default_storage_engine=InnoDB
binlog_format=row
innodb_autoinc_lock_mode=2

# Cluster
wsrep_on=ON
wsrep_cluster_address="gcomm://mariadb-galera-0.mariadb-galera-internal.default.svc.cluster.local,mariadb-galera-1.mariadb-galera-internal.default.svc.cluster.local,mariadb-galera-2.mariadb-galera-internal.default.svc.cluster.local"
wsrep_cluster_name=mariadb-operator
wsrep_slave_threads=1

# Node
wsrep_node_address="2001:db8::a2"
wsrep_node_name="mariadb-galera-1"

# Provider
wsrep_provider=/usr/lib/galera/libgalera_smm.so
wsrep_provider_options="gcache.size=1G;gcs.fc_limit=128;gmcast.listen_addr=tcp://[::]:4567;ist.recv_addr=[2001:db8::a2]:4568"

# SST
wsrep_sst_method="mariabackup"
wsrep_sst_auth="root:mariadb"
wsrep_sst_receive_address="[2001:db8::a2]:4444"`),
			false,
		),
	)
})
