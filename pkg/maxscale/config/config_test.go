package config

import (
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/mariadb/v1alpha1"
	"reflect"
	"sort"
	"strings"
	"testing"

	"k8s.io/utils/ptr"
)

func TestMaxScaleConfig(t *testing.T) {
	tests := []struct {
		name       string
		mxs        *mariadbv1alpha1.MaxScale
		wantConfig string
	}{
		{
			name: "default",
			mxs: &mariadbv1alpha1.MaxScale{
				Spec: mariadbv1alpha1.MaxScaleSpec{
					Admin: mariadbv1alpha1.MaxScaleAdmin{
						Port: 8989,
					},
				},
			},
			wantConfig: `[maxscale]
threads=auto
persist_runtime_changes=true
load_persisted_configs=true
admin_host=0.0.0.0
admin_port=8989
admin_gui=true
admin_secure_gui=false
`,
		},
		{
			name: "tls",
			mxs: &mariadbv1alpha1.MaxScale{
				Spec: mariadbv1alpha1.MaxScaleSpec{
					Admin: mariadbv1alpha1.MaxScaleAdmin{
						Port: 8989,
					},
					TLS: &mariadbv1alpha1.MaxScaleTLS{
						Enabled: true,
					},
				},
			},
			wantConfig: `[maxscale]
threads=auto
persist_runtime_changes=true
load_persisted_configs=true
admin_host=0.0.0.0
admin_port=8989
admin_gui=true
admin_secure_gui=true
admin_ssl_key=/etc/pki/admin.key
admin_ssl_cert=/etc/pki/admin.crt
admin_ssl_ca_cert=/etc/pki/ca.crt
`,
		},
		{
			name: "extra params",
			mxs: &mariadbv1alpha1.MaxScale{
				Spec: mariadbv1alpha1.MaxScaleSpec{
					Config: mariadbv1alpha1.MaxScaleConfig{
						Params: map[string]string{
							"log_info":   "true",
							"logdir":     "/var/log/maxscale/",
							"datadir":    "/var/lib/maxscale/",
							"persistdir": "/var/lib/maxscale/maxscale.cnf.d/",
						},
					},
					Admin: mariadbv1alpha1.MaxScaleAdmin{
						Port:       8989,
						GuiEnabled: ptr.To(false),
					},
				},
			},
			wantConfig: `[maxscale]
threads=auto
persist_runtime_changes=true
load_persisted_configs=true
admin_host=0.0.0.0
admin_port=8989
admin_gui=false
admin_secure_gui=false
log_info=true
logdir=/var/log/maxscale/
datadir=/var/lib/maxscale/
persistdir=/var/lib/maxscale/maxscale.cnf.d/
`,
		},
		{
			name: "override params",
			mxs: &mariadbv1alpha1.MaxScale{
				Spec: mariadbv1alpha1.MaxScaleSpec{
					Config: mariadbv1alpha1.MaxScaleConfig{
						Params: map[string]string{
							"threads":    "4",
							"datadir":    "/var/lib/maxscale/",
							"persistdir": "/var/lib/maxscale/maxscale.cnf.d/",
						},
					},
					Admin: mariadbv1alpha1.MaxScaleAdmin{
						Port: 8989,
					},
				},
			},
			wantConfig: `[maxscale]
threads=4
persist_runtime_changes=true
load_persisted_configs=true
admin_host=0.0.0.0
admin_port=8989
admin_gui=true
admin_secure_gui=false
datadir=/var/lib/maxscale/
persistdir=/var/lib/maxscale/maxscale.cnf.d/
`,
		},
		{
			name: "override query_classifier_cache_size",
			mxs: &mariadbv1alpha1.MaxScale{
				Spec: mariadbv1alpha1.MaxScaleSpec{
					Config: mariadbv1alpha1.MaxScaleConfig{
						Params: map[string]string{
							"query_classifier_cache_size": "10MB",
						},
					},
					Admin: mariadbv1alpha1.MaxScaleAdmin{
						Port: 8989,
					},
				},
			},
			wantConfig: `[maxscale]
threads=auto
query_classifier_cache_size=10MB
persist_runtime_changes=true
load_persisted_configs=true
admin_host=0.0.0.0
admin_port=8989
admin_gui=true
admin_secure_gui=false
`,
		},
		{
			name: "non overridable params",
			mxs: &mariadbv1alpha1.MaxScale{
				Spec: mariadbv1alpha1.MaxScaleSpec{
					Config: mariadbv1alpha1.MaxScaleConfig{
						Params: map[string]string{
							"threads":                 "4",
							"persist_runtime_changes": "false",
							"load_persisted_configs":  "false",
							"admin_secure_gui":        "true",
						},
					},
					Admin: mariadbv1alpha1.MaxScaleAdmin{
						Port: 8989,
					},
				},
			},
			wantConfig: `[maxscale]
threads=4
persist_runtime_changes=true
load_persisted_configs=true
admin_host=0.0.0.0
admin_port=8989
admin_gui=true
admin_secure_gui=false
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bytes, err := Config(tt.mxs)
			if err != nil {
				t.Error("expect error to have occurred, got nil")
			}
			config := string(bytes)
			wantLines := strings.Split(tt.wantConfig, "\n")
			gotLines := strings.Split(config, "\n")
			// Sort both slices for predictable order, as the parameters might be rendered in different order
			sort.Strings(wantLines)
			sort.Strings(gotLines)
			if !reflect.DeepEqual(wantLines, gotLines) {
				t.Errorf("expecting config to be:\n%v\ngot:\n%v\n", tt.wantConfig, config)
			}
		})
	}
}
