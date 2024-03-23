package config

import (
	"reflect"
	"testing"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/pkg/environment"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
					Galera: &mariadbv1alpha1.Galera{
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
					Galera: &mariadbv1alpha1.Galera{
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

# SST
wsrep_sst_method="rsync"
wsrep_sst_receive_address="10.244.0.32:4444"

# Provider
wsrep_provider=/usr/lib/galera/libgalera_smm.so
wsrep_provider_options="gmcast.listen_addr=tcp://0.0.0.0:4567;ist.recv_addr=10.244.0.32:4568"
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
					Galera: &mariadbv1alpha1.Galera{
						Enabled: true,
						GaleraSpec: mariadbv1alpha1.GaleraSpec{
							SST:            mariadbv1alpha1.SSTMariaBackup,
							GaleraLibPath:  "/usr/lib/galera/libgalera_enterprise_smm.so",
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

# SST
wsrep_sst_method="mariabackup"
wsrep_sst_auth="root:mariadb"
wsrep_sst_receive_address="10.244.0.32:4444"

# Provider
wsrep_provider=/usr/lib/galera/libgalera_enterprise_smm.so
wsrep_provider_options="gmcast.listen_addr=tcp://0.0.0.0:4567;ist.recv_addr=10.244.0.32:4568"
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
					Galera: &mariadbv1alpha1.Galera{
						Enabled: true,
						GaleraSpec: mariadbv1alpha1.GaleraSpec{
							SST:            mariadbv1alpha1.SSTMariaBackup,
							GaleraLibPath:  "/usr/lib/galera/libgalera_enterprise_smm.so",
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

# SST
wsrep_sst_method="mariabackup"
wsrep_sst_auth="root:mariadb"
wsrep_sst_receive_address="[2001:db8::a1]:4444"

# Provider
wsrep_provider=/usr/lib/galera/libgalera_enterprise_smm.so
wsrep_provider_options="gmcast.listen_addr=tcp://[::]:4567;ist.recv_addr=[2001:db8::a1]:4568"
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
					Galera: &mariadbv1alpha1.Galera{
						Enabled: true,
						GaleraSpec: mariadbv1alpha1.GaleraSpec{
							SST:            mariadbv1alpha1.SSTMariaBackup,
							GaleraLibPath:  "/usr/lib/galera/libgalera_enterprise_smm.so",
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

# SST
wsrep_sst_method="mariabackup"
wsrep_sst_auth="root:mariadb"
wsrep_sst_receive_address="[2001:db8::a1]:4444"

# Provider
wsrep_provider=/usr/lib/galera/libgalera_enterprise_smm.so
wsrep_provider_options="gcache.size=1G;gcs.fc_limit=128;gmcast.listen_addr=tcp://[::]:4567;ist.recv_addr=[2001:db8::a1]:4568"
`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := NewConfigFile(tt.mariadb)
			bytes, err := config.Marshal(tt.podEnv)
			if tt.wantErr && err == nil {
				t.Error("error expected, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("error unexpected, got %v", err)
			}
			if tt.wantConfig != string(bytes) {
				t.Errorf("unexpected result:\nexpected:\n%s\ngot:\n%s\n", tt.wantConfig, string(bytes))
			}
		})
	}
}

func TestGaleraConfigUpdate(t *testing.T) {
	tests := []struct {
		name      string
		config    string
		podEnv    *environment.PodEnvironment
		wantBytes []byte
		wantErr   bool
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

# SST
wsrep_sst_method="mariabackup"
wsrep_sst_auth="root:mariadb"
wsrep_sst_receive_address="10.244.0.32:4444"

# Provider
wsrep_provider=/usr/lib/galera/libgalera_enterprise_smm.so
wsrep_provider_options="gmcast.listen_addr=tcp://0.0.0.0:4567;ist.recv_addr=10.244.0.32:4568"`,
			podEnv: &environment.PodEnvironment{
				PodIP: "foo",
			},
			wantBytes: nil,
			wantErr:   true,
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

# SST
wsrep_sst_method="mariabackup"
wsrep_sst_auth="root:mariadb"
wsrep_sst_receive_address="10.244.0.32:4444"

# Provider
wsrep_provider=/usr/lib/galera/libgalera_enterprise_smm.so
wsrep_provider_options="gmcast.listen_addr=tcp://0.0.0.0:4567;ist.recv_addr=10.244.0.32:4568"`,
			podEnv: &environment.PodEnvironment{
				PodIP: "10.244.0.33",
			},
			wantBytes: []byte(`[mariadb]
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

# SST
wsrep_sst_method="mariabackup"
wsrep_sst_auth="root:mariadb"
wsrep_sst_receive_address="10.244.0.33:4444"

# Provider
wsrep_provider=/usr/lib/galera/libgalera_enterprise_smm.so
wsrep_provider_options="gmcast.listen_addr=tcp://0.0.0.0:4567;ist.recv_addr=10.244.0.33:4568"`),
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

# SST
wsrep_sst_method="mariabackup"
wsrep_sst_auth="root:mariadb"
wsrep_sst_receive_address="[2001:db8::a1]:4444"

# Provider
wsrep_provider=/usr/lib/galera/libgalera_enterprise_smm.so
wsrep_provider_options="gcache.size=1G;gcs.fc_limit=128;gmcast.listen_addr=tcp://[::]:4567;ist.recv_addr=[2001:db8::a1]:4568"`,
			podEnv: &environment.PodEnvironment{
				PodIP: "2001:db8::a2",
			},
			wantBytes: []byte(`[mariadb]
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

# SST
wsrep_sst_method="mariabackup"
wsrep_sst_auth="root:mariadb"
wsrep_sst_receive_address="[2001:db8::a2]:4444"

# Provider
wsrep_provider=/usr/lib/galera/libgalera_enterprise_smm.so
wsrep_provider_options="gcache.size=1G;gcs.fc_limit=128;gmcast.listen_addr=tcp://[::]:4567;ist.recv_addr=[2001:db8::a2]:4568"`),
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
			if !reflect.DeepEqual(tt.wantBytes, bytes) {
				t.Errorf("unexpected result:\nexpected:\n%s\ngot:\n%s\n", string(tt.wantBytes), string(bytes))
			}
		})
	}
}
