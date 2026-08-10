package config

import (
	"sort"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	"k8s.io/utils/ptr"
)

var _ = Describe("Config", func() {
	DescribeTable("rendering MaxScale config",
		func(mxs *mariadbv1alpha1.MaxScale, wantConfig string) {
			bytes, err := Config(mxs)
			Expect(err).NotTo(HaveOccurred())
			config := string(bytes)
			wantLines := strings.Split(wantConfig, "\n")
			gotLines := strings.Split(config, "\n")
			// Sort both slices for predictable order, as the parameters might be rendered in different order
			sort.Strings(wantLines)
			sort.Strings(gotLines)
			Expect(gotLines).To(Equal(wantLines))
		},
		Entry("default",
			&mariadbv1alpha1.MaxScale{
				Spec: mariadbv1alpha1.MaxScaleSpec{
					Admin: mariadbv1alpha1.MaxScaleAdmin{
						Port: 8989,
					},
				},
			},
			`[maxscale]
threads=auto
persist_runtime_changes=true
load_persisted_configs=true
admin_host=0.0.0.0
admin_port=8989
admin_gui=true
admin_secure_gui=false
`,
		),
		Entry("tls",
			&mariadbv1alpha1.MaxScale{
				Spec: mariadbv1alpha1.MaxScaleSpec{
					Admin: mariadbv1alpha1.MaxScaleAdmin{
						Port: 8989,
					},
					TLS: &mariadbv1alpha1.MaxScaleTLS{
						Enabled: true,
					},
				},
			},
			`[maxscale]
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
		),
		Entry("extra params",
			&mariadbv1alpha1.MaxScale{
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
			`[maxscale]
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
		),
		Entry("override params",
			&mariadbv1alpha1.MaxScale{
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
			`[maxscale]
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
		),
		Entry("override query_classifier_cache_size",
			&mariadbv1alpha1.MaxScale{
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
			`[maxscale]
threads=auto
query_classifier_cache_size=10MB
persist_runtime_changes=true
load_persisted_configs=true
admin_host=0.0.0.0
admin_port=8989
admin_gui=true
admin_secure_gui=false
`,
		),
		Entry("non overridable params",
			&mariadbv1alpha1.MaxScale{
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
			`[maxscale]
threads=4
persist_runtime_changes=true
load_persisted_configs=true
admin_host=0.0.0.0
admin_port=8989
admin_gui=true
admin_secure_gui=false
`,
		),
	)
})
