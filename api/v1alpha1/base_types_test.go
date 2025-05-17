package v1alpha1

import (
	cmmeta "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

var _ = Describe("Base types", func() {
	Context("When creating an Affinity object", func() {
		DescribeTable(
			"Should default",
			func(
				affinity *AffinityConfig,
				antiAffinityInstances []string,
				wantAffinity *AffinityConfig,
			) {
				affinity.SetDefaults(antiAffinityInstances...)
				Expect(affinity).To(BeEquivalentTo(wantAffinity))
			},
			Entry(
				"Empty",
				&AffinityConfig{},
				[]string{"mariadb", "maxscale"},
				&AffinityConfig{},
			),
			Entry(
				"Anti-affinity disabled",
				&AffinityConfig{
					AntiAffinityEnabled: ptr.To(false),
				},
				[]string{"mariadb", "maxscale"},
				&AffinityConfig{
					AntiAffinityEnabled: ptr.To(false),
				},
			),
			Entry(
				"Already defaulted",
				&AffinityConfig{
					AntiAffinityEnabled: ptr.To(true),
					Affinity: Affinity{
						PodAntiAffinity: &PodAntiAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: []PodAffinityTerm{
								{
									LabelSelector: &LabelSelector{
										MatchExpressions: []LabelSelectorRequirement{
											{
												Key:      "app.kubernetes.io/instance",
												Operator: metav1.LabelSelectorOpIn,
												Values:   []string{"mariadb", "maxscale"},
											},
										},
									},
									TopologyKey: "kubernetes.io/hostname",
								},
							},
						},
					},
				},
				[]string{"mariadb", "maxscale"},
				&AffinityConfig{
					AntiAffinityEnabled: ptr.To(true),
					Affinity: Affinity{
						PodAntiAffinity: &PodAntiAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: []PodAffinityTerm{
								{
									LabelSelector: &LabelSelector{
										MatchExpressions: []LabelSelectorRequirement{
											{
												Key:      "app.kubernetes.io/instance",
												Operator: metav1.LabelSelectorOpIn,
												Values:   []string{"mariadb", "maxscale"},
											},
										},
									},
									TopologyKey: "kubernetes.io/hostname",
								},
							},
						},
					},
				},
			),
			Entry(
				"No instances",
				&AffinityConfig{
					AntiAffinityEnabled: ptr.To(true),
				},
				nil,
				&AffinityConfig{
					AntiAffinityEnabled: ptr.To(true),
				},
			),
			Entry(
				"Single instance",
				&AffinityConfig{
					AntiAffinityEnabled: ptr.To(true),
				},
				[]string{"mariadb"},
				&AffinityConfig{
					AntiAffinityEnabled: ptr.To(true),
					Affinity: Affinity{
						PodAntiAffinity: &PodAntiAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: []PodAffinityTerm{
								{
									LabelSelector: &LabelSelector{
										MatchExpressions: []LabelSelectorRequirement{
											{
												Key:      "app.kubernetes.io/instance",
												Operator: metav1.LabelSelectorOpIn,
												Values:   []string{"mariadb"},
											},
										},
									},
									TopologyKey: "kubernetes.io/hostname",
								},
							},
						},
					},
				},
			),
			Entry(
				"Multiple instances",
				&AffinityConfig{
					AntiAffinityEnabled: ptr.To(true),
				},
				[]string{"mariadb", "maxscale"},
				&AffinityConfig{
					AntiAffinityEnabled: ptr.To(true),
					Affinity: Affinity{
						PodAntiAffinity: &PodAntiAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: []PodAffinityTerm{
								{
									LabelSelector: &LabelSelector{
										MatchExpressions: []LabelSelectorRequirement{
											{
												Key:      "app.kubernetes.io/instance",
												Operator: metav1.LabelSelectorOpIn,
												Values:   []string{"mariadb", "maxscale"},
											},
										},
									},
									TopologyKey: "kubernetes.io/hostname",
								},
							},
						},
					},
				},
			),
			Entry(
				"AntiAffinity and nodeAffinity",
				&AffinityConfig{
					AntiAffinityEnabled: ptr.To(true),
					Affinity: Affinity{
						NodeAffinity: &NodeAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: &NodeSelector{
								NodeSelectorTerms: []NodeSelectorTerm{
									{
										MatchExpressions: []NodeSelectorRequirement{
											{
												Key:      "kubernetes.io/hostname",
												Operator: corev1.NodeSelectorOpIn,
												Values:   []string{"node1", "node2"},
											},
										},
									},
								},
							},
						},
					},
				},
				[]string{"mariadb"},
				&AffinityConfig{
					AntiAffinityEnabled: ptr.To(true),
					Affinity: Affinity{
						PodAntiAffinity: &PodAntiAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: []PodAffinityTerm{
								{
									LabelSelector: &LabelSelector{
										MatchExpressions: []LabelSelectorRequirement{
											{
												Key:      "app.kubernetes.io/instance",
												Operator: metav1.LabelSelectorOpIn,
												Values:   []string{"mariadb"},
											},
										},
									},
									TopologyKey: "kubernetes.io/hostname",
								},
							},
						},
						NodeAffinity: &NodeAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: &NodeSelector{
								NodeSelectorTerms: []NodeSelectorTerm{
									{
										MatchExpressions: []NodeSelectorRequirement{
											{
												Key:      "kubernetes.io/hostname",
												Operator: corev1.NodeSelectorOpIn,
												Values:   []string{"node1", "node2"},
											},
										},
									},
								},
							},
						},
					},
				},
			),
		)
	})

	Context("When merging multiple Metadata instances", func() {
		DescribeTable(
			"Should succeed",
			func(
				metas []*Metadata,
				wantMeta *Metadata,
			) {
				gotMeta := MergeMetadata(metas...)
				Expect(wantMeta).To(BeEquivalentTo(gotMeta))
			},
			Entry(
				"empty",
				[]*Metadata{},
				&Metadata{
					Labels:      map[string]string{},
					Annotations: map[string]string{},
				},
			),
			Entry(
				"single",
				[]*Metadata{
					{
						Labels: map[string]string{
							"database.myorg.io": "mariadb",
						},
						Annotations: map[string]string{
							"database.myorg.io": "mariadb",
						},
					},
				},
				&Metadata{
					Labels: map[string]string{
						"database.myorg.io": "mariadb",
					},
					Annotations: map[string]string{
						"database.myorg.io": "mariadb",
					},
				},
			),
			Entry(
				"multiple",
				[]*Metadata{
					{
						Labels: map[string]string{
							"database.myorg.io": "mariadb",
						},
						Annotations: map[string]string{
							"database.myorg.io": "mariadb",
						},
					},
					{
						Labels: map[string]string{
							"sidecar.istio.io/inject": "false",
						},
						Annotations: map[string]string{
							"sidecar.istio.io/inject": "false",
						},
					},
				},
				&Metadata{
					Labels: map[string]string{
						"database.myorg.io":       "mariadb",
						"sidecar.istio.io/inject": "false",
					},
					Annotations: map[string]string{
						"database.myorg.io":       "mariadb",
						"sidecar.istio.io/inject": "false",
					},
				},
			),
			Entry(
				"override",
				[]*Metadata{
					{
						Labels: map[string]string{
							"database.myorg.io": "mariadb",
						},
						Annotations: map[string]string{
							"database.myorg.io": "mariadb",
						},
					},
					{
						Labels: map[string]string{
							"database.myorg.io": "mydb",
						},
						Annotations: map[string]string{
							"database.myorg.io": "mydb",
						},
					},
				},
				&Metadata{
					Labels: map[string]string{
						"database.myorg.io": "mydb",
					},
					Annotations: map[string]string{
						"database.myorg.io": "mydb",
					},
				},
			),
		)
	})

	Context("When validating TLS", func() {
		DescribeTable(
			"Should validate",
			func(
				item *tlsValidationItem,
				wantErr bool,
			) {
				err := validateTLSCert(item)
				if wantErr {
					Expect(err).To(HaveOccurred())
				} else {
					Expect(err).ToNot(HaveOccurred())
				}
			},
			Entry(
				"empty",
				&tlsValidationItem{},
				false,
			),
			Entry(
				"certSecretRef and caSecretRef",
				&tlsValidationItem{
					certSecretRef: &LocalObjectReference{Name: "cert-secret"},
					caSecretRef:   &LocalObjectReference{Name: "ca-secret"},
				},
				false,
			),
			Entry(
				"certIssuerRef",
				&tlsValidationItem{
					certIssuerRef: &cmmeta.ObjectReference{Name: "cert-issuer"},
				},
				false,
			),
			Entry(
				"certIssuerRef and caSecretRef",
				&tlsValidationItem{
					certIssuerRef: &cmmeta.ObjectReference{Name: "cert-issuer"},
					caSecretRef:   &LocalObjectReference{Name: "ca-secret"},
				},
				false,
			),
			Entry(
				"certSecretRef set without caSecretRef",
				&tlsValidationItem{
					certSecretRef: &LocalObjectReference{Name: "cert-secret"},
				},
				true,
			),
			Entry(
				"certSecretRef and certIssuerRef",
				&tlsValidationItem{
					certSecretRef: &LocalObjectReference{Name: "cert-secret"},
					certIssuerRef: &cmmeta.ObjectReference{Name: "cert-issuer"},
				},
				true,
			),
		)
	})
})
