package v1alpha1

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

var _ = Describe("SqlJob types", func() {
	objMeta := metav1.ObjectMeta{
		Name:      "sqljob-obj",
		Namespace: testNamespace,
	}
	mdbObjMeta := metav1.ObjectMeta{
		Name:      "mdb-sqljob-obj",
		Namespace: testNamespace,
	}
	Context("When creating a SqlJob object", func() {
		DescribeTable(
			"Should default",
			func(sqlJob *SqlJob, mariadb *MariaDB, expectedSqlJob *SqlJob) {
				sqlJob.SetDefaults(mariadb)
				Expect(sqlJob).To(BeEquivalentTo(expectedSqlJob))
			},
			Entry(
				"Empty",
				&SqlJob{
					ObjectMeta: objMeta,
				},
				&MariaDB{
					ObjectMeta: mdbObjMeta,
				},
				&SqlJob{
					ObjectMeta: objMeta,
					Spec: SqlJobSpec{
						JobPodTemplate: JobPodTemplate{
							ServiceAccountName: &objMeta.Name,
						},
						BackoffLimit: 5,
					},
				},
			),
			Entry(
				"Anti affinity",
				&SqlJob{
					ObjectMeta: objMeta,
					Spec: SqlJobSpec{
						JobPodTemplate: JobPodTemplate{
							Affinity: &AffinityConfig{
								AntiAffinityEnabled: ptr.To(true),
							},
						},
					},
				},
				&MariaDB{
					ObjectMeta: mdbObjMeta,
				},
				&SqlJob{
					ObjectMeta: objMeta,
					Spec: SqlJobSpec{
						JobPodTemplate: JobPodTemplate{
							ServiceAccountName: &objMeta.Name,
							Affinity: &AffinityConfig{
								AntiAffinityEnabled: ptr.To(true),
								Affinity: corev1.Affinity{
									PodAntiAffinity: &corev1.PodAntiAffinity{
										RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
											{
												LabelSelector: &metav1.LabelSelector{
													MatchExpressions: []metav1.LabelSelectorRequirement{
														{
															Key:      "app.kubernetes.io/instance",
															Operator: metav1.LabelSelectorOpIn,
															Values: []string{
																mdbObjMeta.Name,
															},
														},
													},
												},
												TopologyKey: "kubernetes.io/hostname",
											},
										},
									},
								},
							},
						},
						BackoffLimit: 5,
					},
				},
			),
			Entry(
				"Full",
				&SqlJob{
					ObjectMeta: objMeta,
					Spec: SqlJobSpec{
						JobPodTemplate: JobPodTemplate{
							ServiceAccountName: ptr.To("sqljob-test"),
							Affinity: &AffinityConfig{
								AntiAffinityEnabled: ptr.To(true),
							},
						},
						BackoffLimit: 3,
					},
				},
				&MariaDB{
					ObjectMeta: mdbObjMeta,
				},
				&SqlJob{
					ObjectMeta: objMeta,
					Spec: SqlJobSpec{
						JobPodTemplate: JobPodTemplate{
							ServiceAccountName: ptr.To("sqljob-test"),
							Affinity: &AffinityConfig{
								AntiAffinityEnabled: ptr.To(true),
								Affinity: corev1.Affinity{
									PodAntiAffinity: &corev1.PodAntiAffinity{
										RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
											{
												LabelSelector: &metav1.LabelSelector{
													MatchExpressions: []metav1.LabelSelectorRequirement{
														{
															Key:      "app.kubernetes.io/instance",
															Operator: metav1.LabelSelectorOpIn,
															Values: []string{
																mdbObjMeta.Name,
															},
														},
													},
												},
												TopologyKey: "kubernetes.io/hostname",
											},
										},
									},
								},
							},
						},
						BackoffLimit: 3,
					},
				},
			),
		)
	})
})
