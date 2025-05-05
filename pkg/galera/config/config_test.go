package config

import (
	"github.com/mariadb-operator/mariadb-operator/api/mariadb/v1alpha1"
	"testing"

	"github.com/go-logr/logr"
	"github.com/google/go-cmp/cmp"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/mariadb/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/pkg/environment"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func TestGaleraConfigMarshal(t *testing.T) {
	tests := []struct {
		name       string
		mariadb    *mariadbv1alpha1.MariaDB
		podEnv     *environment.PodEnvironment
		wantConfig string
		wantErr    bool
	}{
		{
			name: "no replicas",
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: v1.ObjectMeta{
					Name:      "mariadb-galera",
					Namespace: "default",
				},
				Spec: mariadbv1alpha1.MariaDBSpec{
					Galera: &v1alpha1.Galera{
						Enabled: true,
						GaleraSpec: v1alpha1.GaleraSpec{
							SST:            v1alpha1.SSTRsync,
							ReplicaThreads: 1,
						},
					},
					Replicas: 0,
				},
			},
			podEnv: &environment.PodEnvironment{
				PodName:             "mariadb-galera-0",
				PodIP:               "10.244.0.32",
				MariadbRootPassword: "mariadb",
			},
			wantConfig: "",
			wantErr:    true,
		},
		{
			name: "Galera not enabled",
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: v1.ObjectMeta{
					Name:      "mariadb-galera",
					Namespace: "default",
				},
				Spec: mariadbv1alpha1.MariaDBSpec{
					Galera: &v1alpha1.Galera{
						Enabled: false,
					},
					Replicas: 0,
				},
			},
			podEnv: &environment.PodEnvironment{
				PodName:             "mariadb-galera-0",
				PodIP:               "10.244.0.32",
				MariadbRootPassword: "mariadb",
			},
			wantConfig: "",
			wantErr:    true,
		},
		{
			name: "invalid IP",
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: v1.ObjectMeta{
					Name:      "mariadb-galera",
					Namespace: "default",
				},
				Spec: mariadbv1alpha1.MariaDBSpec{
					Galera: &v1alpha1.Galera{
						Enabled: true,
					},
					Replicas: 0,
				},
			},
			podEnv: &environment.PodEnvironment{
				PodName:             "mariadb-galera-0",
				PodIP:               "foo",
				MariadbRootPassword: "mariadb",
			},
			wantConfig: "",
			wantErr:    true,
		},
		{
			name: "rsync",
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: v1.ObjectMeta{
					Name:      "mariadb-galera",
					Namespace: "default",
				},
				Spec: mariadbv1alpha1.MariaDBSpec{
					Galera: &v1alpha1.Galera{
						Enabled: true,
						GaleraSpec: v1alpha1.GaleraSpec{
							SST:            v1alpha1.SSTRsync,
							GaleraLibPath:  "/usr/lib/galera/libgalera_smm.so",
							ReplicaThreads: 1,
						},
					},
					Replicas: 3,
				},
			},
			podEnv: &environment.PodEnvironment{
				PodName:             "mariadb-galera-0",
				PodIP:               "10.244.0.32",
				MariadbRootPassword: "mariadb",
			},
			//nolint:lll
			wantConfig: `[mariadb]
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
			wantErr: false,
		},
		{
			name: "mariabackup",
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: v1.ObjectMeta{
					Name:      "mariadb-galera",
					Namespace: "default",
				},
				Spec: mariadbv1alpha1.MariaDBSpec{
					Galera: &v1alpha1.Galera{
						Enabled: true,
						GaleraSpec: v1alpha1.GaleraSpec{
							SST:            v1alpha1.SSTMariaBackup,
							GaleraLibPath:  "/usr/lib/galera/libgalera_smm.so",
							ReplicaThreads: 2,
						},
					},
					Replicas: 3,
				},
			},
			podEnv: &environment.PodEnvironment{
				PodName:             "mariadb-galera-1",
				PodIP:               "10.244.0.32",
				MariadbRootPassword: "mariadb",
			},
			//nolint:lll
			wantConfig: `[mariadb]
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
			wantErr: false,
		},
		{
			name: "IPv6",
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: v1.ObjectMeta{
					Name:      "mariadb-galera",
					Namespace: "default",
				},
				Spec: mariadbv1alpha1.MariaDBSpec{
					Galera: &v1alpha1.Galera{
						Enabled: true,
						GaleraSpec: v1alpha1.GaleraSpec{
							SST:            v1alpha1.SSTMariaBackup,
							GaleraLibPath:  "/usr/lib/galera/libgalera_smm.so",
							ReplicaThreads: 1,
						},
					},
					Replicas: 3,
				},
			},
			podEnv: &environment.PodEnvironment{
				PodName:             "mariadb-galera-1",
				PodIP:               "2001:db8::a1",
				MariadbRootPassword: "mariadb",
			},
			//nolint:lll
			wantConfig: `[mariadb]
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
			wantErr: false,
		},
		{
			name: "Additional WSREP provider options",
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: v1.ObjectMeta{
					Name:      "mariadb-galera",
					Namespace: "default",
				},
				Spec: mariadbv1alpha1.MariaDBSpec{
					Galera: &v1alpha1.Galera{
						Enabled: true,
						GaleraSpec: v1alpha1.GaleraSpec{
							SST:            v1alpha1.SSTMariaBackup,
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
			podEnv: &environment.PodEnvironment{
				PodName:             "mariadb-galera-1",
				PodIP:               "2001:db8::a1",
				MariadbRootPassword: "mariadb",
			},
			//nolint:lll
			wantConfig: `[mariadb]
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
			wantErr: false,
		},
		{
			name: "TLS",
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: v1.ObjectMeta{
					Name:      "mariadb-galera",
					Namespace: "default",
				},
				Spec: mariadbv1alpha1.MariaDBSpec{
					Image: "mariadb:10.11.8",
					Galera: &v1alpha1.Galera{
						Enabled: true,
						GaleraSpec: v1alpha1.GaleraSpec{
							SST:            v1alpha1.SSTMariaBackup,
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
			podEnv: &environment.PodEnvironment{
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
			wantConfig: `[mariadb]
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
			wantErr: false,
		},
		{
			name: "TLS with Galera SST disabled",
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: v1.ObjectMeta{
					Name:      "mariadb-galera",
					Namespace: "default",
				},
				Spec: mariadbv1alpha1.MariaDBSpec{
					Image: "mariadb:10.11.8",
					Galera: &v1alpha1.Galera{
						Enabled: true,
						GaleraSpec: v1alpha1.GaleraSpec{
							SST:            v1alpha1.SSTMariaBackup,
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
			podEnv: &environment.PodEnvironment{
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
			wantConfig: `[mariadb]
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
			wantErr: false,
		},
		{
			name: "TLS with required disabled",
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: v1.ObjectMeta{
					Name:      "mariadb-galera",
					Namespace: "default",
				},
				Spec: mariadbv1alpha1.MariaDBSpec{
					Image: "mariadb:10.11.8",
					Galera: &v1alpha1.Galera{
						Enabled: true,
						GaleraSpec: v1alpha1.GaleraSpec{
							SST:            v1alpha1.SSTMariaBackup,
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
			podEnv: &environment.PodEnvironment{
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
			wantConfig: `[mariadb]
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
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bytes, err := NewConfigFile(tt.mariadb, logr.Discard()).Marshal(tt.podEnv)

			if tt.wantErr && err == nil {
				t.Error("expect error to have occurred, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("expect error to not have occurred, got: %v", err)
			}
			if diff := cmp.Diff(tt.wantConfig, string(bytes)); diff != "" {
				t.Errorf("unexpected config (-want +got):\n%s", diff)
			}
		})
	}
}

func TestGaleraConfigUpdate(t *testing.T) {
	tests := []struct {
		name       string
		config     string
		podEnv     *environment.PodEnvironment
		wantConfig []byte
		wantErr    bool
	}{
		{
			name: "invalid IP",
			config: `[mariadb]
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
			podEnv: &environment.PodEnvironment{
				PodIP: "foo",
			},
			wantConfig: nil,
			wantErr:    true,
		},
		{
			name: "IPv4",
			config: `[mariadb]
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
			podEnv: &environment.PodEnvironment{
				PodIP: "10.244.0.33",
			},
			wantConfig: []byte(`[mariadb]
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
			wantErr: false,
		},
		{
			name: "IPv6",
			config: `[mariadb]
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
			podEnv: &environment.PodEnvironment{
				PodIP: "2001:db8::a2",
			},
			wantConfig: []byte(`[mariadb]
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
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bytes, err := UpdateConfig([]byte(tt.config), tt.podEnv)
			if tt.wantErr && err == nil {
				t.Error("error expected, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("error unexpected, got %v", err)
			}
			if diff := cmp.Diff(string(tt.wantConfig), string(bytes)); diff != "" {
				t.Errorf("unexpected config (-want +got):\n%s", diff)
			}
		})
	}
}
