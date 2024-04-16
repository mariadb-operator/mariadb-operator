package v1alpha1

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

var _ = Describe("Restore types", func() {
	objMeta := metav1.ObjectMeta{
		Name:      "restore-obj",
		Namespace: testNamespace,
	}
	Context("When creating a Restore object", func() {
		DescribeTable(
			"Should default",
			func(backup, expected *Restore) {
				backup.SetDefaults()
				Expect(backup).To(BeEquivalentTo(expected))
			},
			Entry(
				"Empty",
				&Restore{
					ObjectMeta: objMeta,
				},
				&Restore{
					ObjectMeta: objMeta,
					Spec: RestoreSpec{
						JobPodTemplate: JobPodTemplate{
							ServiceAccountName: &objMeta.Name,
						},
						BackoffLimit: 5,
					},
				},
			),
			Entry(
				"Full",
				&Restore{
					ObjectMeta: objMeta,
					Spec: RestoreSpec{
						JobPodTemplate: JobPodTemplate{
							ServiceAccountName: ptr.To("restore-test"),
						},
						BackoffLimit: 3,
					},
				},
				&Restore{
					ObjectMeta: objMeta,
					Spec: RestoreSpec{
						JobPodTemplate: JobPodTemplate{
							ServiceAccountName: ptr.To("restore-test"),
						},
						BackoffLimit: 3,
					},
				},
			),
		)
	})
})
