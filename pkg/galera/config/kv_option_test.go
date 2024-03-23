package config

import (
	"reflect"
	"testing"
)

func TestKvOptionMarshal(t *testing.T) {
	tests := []struct {
		name       string
		kvOpt      *kvOption
		wantString string
	}{
		{
			name:       "unquoted",
			kvOpt:      newKvOption("gmcast.listen_addr", "tcp://0.0.0.0:4567", false),
			wantString: "gmcast.listen_addr=tcp://0.0.0.0:4567",
		},
		{
			name:       "unquoted with multiple values",
			kvOpt:      newKvOption("wsrep_provider_options", "gcache.size=1G;gcs.fc_limit=128;gmcast.listen_addr=tcp://[::]:4567;ist.recv_addr=[2001:db8::a1]:4568", false),
			wantString: "wsrep_provider_options=gcache.size=1G;gcs.fc_limit=128;gmcast.listen_addr=tcp://[::]:4567;ist.recv_addr=[2001:db8::a1]:4568",
		},
		{
			name:       "quoted",
			kvOpt:      newKvOption("wsrep_node_address", "10.244.0.33", true),
			wantString: "wsrep_node_address=\"10.244.0.33\"",
		},
		{
			name:       "quoted with multiple values",
			kvOpt:      newKvOption("wsrep_provider_options", "gcache.size=1G;gcs.fc_limit=128;gmcast.listen_addr=tcp://[::]:4567;ist.recv_addr=[2001:db8::a1]:4568", true),
			wantString: "wsrep_provider_options=\"gcache.size=1G;gcs.fc_limit=128;gmcast.listen_addr=tcp://[::]:4567;ist.recv_addr=[2001:db8::a1]:4568\"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotString := tt.kvOpt.marshal()
			if tt.wantString != gotString {
				t.Errorf("unexpected result:\nexpected:\n%s\ngot:\n%s\n", tt.wantString, gotString)
			}
		})
	}
}

func TestKvOptionUnmarshal(t *testing.T) {
	tests := []struct {
		name      string
		rawKvOpt  string
		wantKvOpt kvOption
		wantErr   bool
	}{
		{
			name:      "empty",
			rawKvOpt:  " ",
			wantKvOpt: kvOption{},
			wantErr:   true,
		},
		{
			name:      "invalid",
			rawKvOpt:  "wsrep_node_address: 2001:db8::a2",
			wantKvOpt: kvOption{},
			wantErr:   true,
		},
		{
			name:     "unquoted",
			rawKvOpt: "wsrep_cluster_name=mariadb-operator",
			wantKvOpt: kvOption{
				key:    "wsrep_cluster_name",
				value:  "mariadb-operator",
				quoted: false,
			},
			wantErr: false,
		},
		{
			name:     "quoted with multiple values",
			rawKvOpt: "wsrep_provider_options=gcache.size=1G;gcs.fc_limit=128;gmcast.listen_addr=tcp://[::]:4567;ist.recv_addr=[2001:db8::a1]:4568",
			wantKvOpt: kvOption{
				key:    "wsrep_provider_options",
				value:  "gcache.size=1G;gcs.fc_limit=128;gmcast.listen_addr=tcp://[::]:4567;ist.recv_addr=[2001:db8::a1]:4568",
				quoted: false,
			},
			wantErr: false,
		},
		{
			name:     "quoted",
			rawKvOpt: "wsrep_sst_receive_address=\"[2001:db8::a2]:4444\"",
			wantKvOpt: kvOption{
				key:    "wsrep_sst_receive_address",
				value:  "[2001:db8::a2]:4444",
				quoted: true,
			},
			wantErr: false,
		},
		{
			name:     "quoted with multiple values",
			rawKvOpt: "wsrep_provider_options=\"gcache.size=1G;gcs.fc_limit=128;gmcast.listen_addr=tcp://[::]:4567;ist.recv_addr=[2001:db8::a1]:4568\"",
			wantKvOpt: kvOption{
				key:    "wsrep_provider_options",
				value:  "gcache.size=1G;gcs.fc_limit=128;gmcast.listen_addr=tcp://[::]:4567;ist.recv_addr=[2001:db8::a1]:4568",
				quoted: true,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var kvOpt kvOption
			err := kvOpt.unmarshal(tt.rawKvOpt)
			if !tt.wantErr && err != nil {
				t.Errorf("error unexpected, got %v", err)
			}
			if !reflect.DeepEqual(tt.wantKvOpt, kvOpt) {
				t.Errorf("unexpected result:\nexpected:\n%v\ngot:\n%v\n", tt.wantKvOpt, kvOpt)
			}
		})
	}
}
