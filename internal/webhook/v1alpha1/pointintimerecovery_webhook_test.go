package v1alpha1

import (
	"github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("PointInTimeRecovery Webhook", func() {
	Context("When updating a PointInTimeRecovery", Ordered, func() {
		key := types.NamespacedName{
			Name:      "pitr-update",
			Namespace: testNamespace,
		}
		BeforeAll(func() {
			pitr := v1alpha1.PointInTimeRecovery{
				ObjectMeta: metav1.ObjectMeta{
					Name:      key.Name,
					Namespace: key.Namespace,
				},
				Spec: v1alpha1.PointInTimeRecoverySpec{
					Compression: v1alpha1.CompressGzip,
				},
			}
			Expect(k8sClient.Create(testCtx, &pitr)).To(Succeed())
		})

		DescribeTable(
			"Should validate",
			func(patchFn func(pitr *v1alpha1.PointInTimeRecovery), wantErr bool) {
				var pitr v1alpha1.PointInTimeRecovery
				Expect(k8sClient.Get(testCtx, key, &pitr)).To(Succeed())

				patch := client.MergeFrom(pitr.DeepCopy())
				patchFn(&pitr)

				err := k8sClient.Patch(testCtx, &pitr, patch)
				if wantErr {
					Expect(err).To(HaveOccurred())
				} else {
					Expect(err).ToNot(HaveOccurred())
				}
			},
			Entry(
				"Updating Compression",
				func(pitr *v1alpha1.PointInTimeRecovery) {
					pitr.Spec.Compression = v1alpha1.CompressBzip2
				},
				true,
			),
			Entry(
				"Unsetting Compression",
				func(pitr *v1alpha1.PointInTimeRecovery) {
					pitr.Spec.Compression = ""
				},
				true,
			),
		)
	})
})
