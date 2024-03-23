package config

import (
	"reflect"
	"testing"
)

func TestProviderOptsMarshal(t *testing.T) {
	tests := []struct {
		name         string
		providerOpts *providerOptions
		wantString   string
	}{
		{
			name:         "empty",
			providerOpts: newProviderOptions(map[string]string{}),
			wantString:   "",
		},
		{
			name: "single option",
			providerOpts: newProviderOptions(map[string]string{
				"gmcast.listen_addr": "tcp://0.0.0.0:4567",
			}),
			wantString: "gmcast.listen_addr=tcp://0.0.0.0:4567",
		},
		{
			name: "multiple options",
			providerOpts: newProviderOptions(map[string]string{
				"gcache.size":   "1G",
				"gcs.fc_limit":  "128",
				"ist.recv_addr": "[2001:db8::a1]:4568",
			}),
			wantString: "gcache.size=1G;gcs.fc_limit=128;ist.recv_addr=[2001:db8::a1]:4568",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotString := tt.providerOpts.marshal()
			if tt.wantString != gotString {
				t.Errorf("unexpected result:\nexpected:\n%s\ngot:\n%s\n", tt.wantString, gotString)
			}
		})
	}
}

func TestProviderOptsUnmarshal(t *testing.T) {
	tests := []struct {
		name             string
		rawProviderOpts  string
		wantProviderOpts providerOptions
		wantErr          bool
	}{
		{
			name:             "empty",
			rawProviderOpts:  " ",
			wantProviderOpts: providerOptions{},
			wantErr:          true,
		},
		{
			name:            "single invalid option",
			rawProviderOpts: "gmcast.listen_addr",
			wantProviderOpts: providerOptions{
				make(map[string]string, 0),
			},
			wantErr: true,
		},
		{
			name:            "multiple invalid options",
			rawProviderOpts: "gmcast.listen_addr;ist.recv_addr",
			wantProviderOpts: providerOptions{
				make(map[string]string, 0),
			},
			wantErr: true,
		},
		{
			name:            "some invalid options",
			rawProviderOpts: "gmcast.listen_addr=tcp://[::]:4567;gcs.fc_limit;ist.recv_addr",
			wantProviderOpts: providerOptions{
				opts: map[string]string{
					"gmcast.listen_addr": "tcp://[::]:4567",
				},
			},
			wantErr: true,
		},
		{
			name:            "single option",
			rawProviderOpts: "gmcast.listen_addr=tcp://[::]:4567",
			wantProviderOpts: providerOptions{
				opts: map[string]string{
					"gmcast.listen_addr": "tcp://[::]:4567",
				},
			},
			wantErr: false,
		},
		{
			name:            "multiple options",
			rawProviderOpts: "gcache.size=1G;gcs.fc_limit=128;gmcast.listen_addr=tcp://[::]:4567;ist.recv_addr=[2001:db8::a1]:4568",
			wantProviderOpts: providerOptions{
				opts: map[string]string{
					"gcache.size":        "1G",
					"gcs.fc_limit":       "128",
					"gmcast.listen_addr": "tcp://[::]:4567",
					"ist.recv_addr":      "[2001:db8::a1]:4568",
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var providerOpts providerOptions
			err := providerOpts.unmarshal(tt.rawProviderOpts)
			if !tt.wantErr && err != nil {
				t.Errorf("error unexpected, got %v", err)
			}
			if !reflect.DeepEqual(tt.wantProviderOpts, providerOpts) {
				t.Errorf("unexpected result:\nexpected:\n%v\ngot:\n%v\n", tt.wantProviderOpts, providerOpts)
			}
		})
	}
}
