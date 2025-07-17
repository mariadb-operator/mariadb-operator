package v1alpha1

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
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
			func(sqlJob *SQLJob, mariadb *MariaDB, expectedSqlJob *SQLJob) {
				sqlJob.SetDefaults(mariadb)
				Expect(sqlJob).To(BeEquivalentTo(expectedSqlJob))
			},
			Entry(
				"Empty",
				&SQLJob{
					ObjectMeta: objMeta,
				},
				&MariaDB{
					ObjectMeta: mdbObjMeta,
				},
				&SQLJob{
					ObjectMeta: objMeta,
					Spec: SQLJobSpec{
						JobPodTemplate: JobPodTemplate{
							ServiceAccountName: &objMeta.Name,
						},
						BackoffLimit: 5,
					},
				},
			),
			Entry(
				"Anti affinity",
				&SQLJob{
					ObjectMeta: objMeta,
					Spec: SQLJobSpec{
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
				&SQLJob{
					ObjectMeta: objMeta,
					Spec: SQLJobSpec{
						JobPodTemplate: JobPodTemplate{
							ServiceAccountName: &objMeta.Name,
							Affinity: &AffinityConfig{
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
				&SQLJob{
					ObjectMeta: objMeta,
					Spec: SQLJobSpec{
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
				&SQLJob{
					ObjectMeta: objMeta,
					Spec: SQLJobSpec{
						JobPodTemplate: JobPodTemplate{
							ServiceAccountName: ptr.To("sqljob-test"),
							Affinity: &AffinityConfig{
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
