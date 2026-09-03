package v1alpha1

import (
	"encoding/json"

	cmmeta "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Base types", func() {
	Context("When round-tripping a custom container", func() {
		It("Should preserve inheritance and the complete restricted security context", func() {
			container := Container{
				Image: "busybox:1.36",
				Inheritance: &ContainerInheritance{
					Policy:       ContainerInheritanceSelected,
					Env:          []ContainerEnvGroup{ContainerEnvGroupRuntime},
					VolumeMounts: []ContainerVolumeMountGroup{ContainerVolumeMountGroupConfig},
				},
				SecurityContext: &SecurityContext{
					RunAsNonRoot:             ptr.To(true),
					AllowPrivilegeEscalation: ptr.To(false),
					Privileged:               ptr.To(false),
					ReadOnlyRootFilesystem:   ptr.To(true),
					Capabilities: &corev1.Capabilities{
						Drop: []corev1.Capability{"ALL"},
					},
					SeccompProfile: &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault},
				},
			}

			data, err := json.Marshal(container)
			Expect(err).ToNot(HaveOccurred())
			var roundTrip Container
			Expect(json.Unmarshal(data, &roundTrip)).To(Succeed())
			Expect(roundTrip).To(Equal(container))

			copied := container.DeepCopy()
			copied.Inheritance.Env[0] = ContainerEnvGroupTLS
			copied.SecurityContext.Capabilities.Drop[0] = "NET_ADMIN"
			Expect(container.Inheritance.Env).To(Equal([]ContainerEnvGroup{ContainerEnvGroupRuntime}))
			Expect(container.SecurityContext.Capabilities.Drop).To(Equal([]corev1.Capability{"ALL"}))
		})

		It("Should preserve omitted inheritance when an old object passes through the API server", func() {
			mariadb := &MariaDB{
				TypeMeta: metav1.TypeMeta{APIVersion: GroupVersion.String(), Kind: "MariaDB"},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "custom-container-legacy-round-trip",
					Namespace: testNamespace,
				},
				Spec: MariaDBSpec{MariaDBPodTemplate: MariaDBPodTemplate{
					SidecarContainers: []Container{{Name: "legacy", Image: "busybox:1.36"}},
				}},
			}
			_ = k8sClient.Delete(testCtx, mariadb)
			Expect(k8sClient.Create(testCtx, mariadb)).To(Succeed())
			DeferCleanup(func() {
				Expect(client.IgnoreNotFound(k8sClient.Delete(testCtx, mariadb))).To(Succeed())
			})

			persisted := &MariaDB{}
			Expect(k8sClient.Get(testCtx, client.ObjectKeyFromObject(mariadb), persisted)).To(Succeed())
			Expect(persisted.Spec.SidecarContainers).To(HaveLen(1))
			Expect(persisted.Spec.SidecarContainers[0].Inheritance).To(BeNil())
		})

		It("Should preserve selected inheritance and security context through server-side apply", func() {
			mariadb := &MariaDB{
				TypeMeta: metav1.TypeMeta{APIVersion: GroupVersion.String(), Kind: "MariaDB"},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "custom-container-selected-apply",
					Namespace: testNamespace,
				},
				Spec: MariaDBSpec{MariaDBPodTemplate: MariaDBPodTemplate{
					SidecarContainers: []Container{{
						Name:  "selected",
						Image: "busybox:1.36",
						Inheritance: &ContainerInheritance{
							Policy:       ContainerInheritanceSelected,
							Env:          []ContainerEnvGroup{ContainerEnvGroupTLS, ContainerEnvGroupRuntime},
							VolumeMounts: []ContainerVolumeMountGroup{ContainerVolumeMountGroupTLS, ContainerVolumeMountGroupConfig},
						},
						SecurityContext: &SecurityContext{
							RunAsNonRoot:             ptr.To(true),
							AllowPrivilegeEscalation: ptr.To(false),
							SeccompProfile: &corev1.SeccompProfile{
								Type: corev1.SeccompProfileTypeRuntimeDefault,
							},
						},
					}},
				}},
			}
			_ = k8sClient.Delete(testCtx, mariadb)
			object, err := runtime.DefaultUnstructuredConverter.ToUnstructured(mariadb)
			Expect(err).ToNot(HaveOccurred())
			applyConfiguration := client.ApplyConfigurationFromUnstructured(&unstructured.Unstructured{Object: object})
			Expect(k8sClient.Apply(
				testCtx,
				applyConfiguration,
				client.FieldOwner("custom-container-inheritance-test"),
			)).To(Succeed())
			DeferCleanup(func() {
				Expect(client.IgnoreNotFound(k8sClient.Delete(testCtx, mariadb))).To(Succeed())
			})

			persisted := &MariaDB{}
			Expect(k8sClient.Get(testCtx, client.ObjectKeyFromObject(mariadb), persisted)).To(Succeed())
			Expect(persisted.Spec.SidecarContainers).To(HaveLen(1))
			container := persisted.Spec.SidecarContainers[0]
			Expect(container.Inheritance).ToNot(BeNil())
			Expect(container.Inheritance.Policy).To(Equal(ContainerInheritanceSelected))
			Expect(container.Inheritance.Env).To(ConsistOf(ContainerEnvGroupRuntime, ContainerEnvGroupTLS))
			Expect(container.Inheritance.VolumeMounts).To(ConsistOf(ContainerVolumeMountGroupConfig, ContainerVolumeMountGroupTLS))
			Expect(container.SecurityContext).ToNot(BeNil())
			Expect(container.SecurityContext.SeccompProfile).ToNot(BeNil())
			Expect(container.SecurityContext.SeccompProfile.Type).To(Equal(corev1.SeccompProfileTypeRuntimeDefault))
		})
	})

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
					certIssuerRef: &cmmeta.IssuerReference{Name: "cert-issuer"},
				},
				false,
			),
			Entry(
				"certIssuerRef and caSecretRef",
				&tlsValidationItem{
					certIssuerRef: &cmmeta.IssuerReference{Name: "cert-issuer"},
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
					certIssuerRef: &cmmeta.IssuerReference{Name: "cert-issuer"},
				},
				true,
			),
		)
	})
})
