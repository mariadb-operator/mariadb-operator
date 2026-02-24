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
	Context("When creating PointInTimeRecovery", func() {
		key := types.NamespacedName{
			Name:      "pitr-storage",
			Namespace: testNamespace,
		}

		DescribeTable(
			"Should validate",
			func(pitr *v1alpha1.PointInTimeRecovery, wantErr bool) {
				_ = k8sClient.Delete(testCtx, pitr)
				err := k8sClient.Create(testCtx, pitr)
				if wantErr {
					Expect(err).To(HaveOccurred())
				} else {
					Expect(err).ToNot(HaveOccurred())
				}
			},
			Entry(
				"No Storage",
				&v1alpha1.PointInTimeRecovery{
					ObjectMeta: metav1.ObjectMeta{
						Name:      key.Name,
						Namespace: key.Namespace,
					},
					Spec: v1alpha1.PointInTimeRecoverySpec{
						Compression: v1alpha1.CompressGzip,
					},
				},
				true,
			),

			Entry(
				"Both ABS and S3",
				&v1alpha1.PointInTimeRecovery{
					ObjectMeta: metav1.ObjectMeta{
						Name:      key.Name,
						Namespace: key.Namespace,
					},
					Spec: v1alpha1.PointInTimeRecoverySpec{
						Compression: v1alpha1.CompressGzip,
						PointInTimeRecoveryStorage: v1alpha1.PointInTimeRecoveryStorage{
							S3:        &v1alpha1.S3{},
							AzureBlob: &v1alpha1.AzureBlob{},
						},
					},
				},
				true,
			),

			Entry(
				"With S3",
				&v1alpha1.PointInTimeRecovery{
					ObjectMeta: metav1.ObjectMeta{
						Name:      key.Name,
						Namespace: key.Namespace,
					},
					Spec: v1alpha1.PointInTimeRecoverySpec{
						Compression: v1alpha1.CompressGzip,
						PointInTimeRecoveryStorage: v1alpha1.PointInTimeRecoveryStorage{
							S3: &v1alpha1.S3{},
						},
					},
				},
				false,
			),

			Entry(
				"With ABS",
				&v1alpha1.PointInTimeRecovery{
					ObjectMeta: metav1.ObjectMeta{
						Name:      key.Name,
						Namespace: key.Namespace,
					},
					Spec: v1alpha1.PointInTimeRecoverySpec{
						Compression: v1alpha1.CompressGzip,
						PointInTimeRecoveryStorage: v1alpha1.PointInTimeRecoveryStorage{
							AzureBlob: &v1alpha1.AzureBlob{},
						},
					},
				},
				false,
			),
		)
	})

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
					PointInTimeRecoveryStorage: v1alpha1.PointInTimeRecoveryStorage{
						S3: &v1alpha1.S3{},
					},
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
			Entry(
				"Adding ABS",
				func(pitr *v1alpha1.PointInTimeRecovery) {
					pitr.Spec.PointInTimeRecoveryStorage.AzureBlob = &v1alpha1.AzureBlob{}
				},
				true,
			),
			Entry(
				"No storage",
				func(pitr *v1alpha1.PointInTimeRecovery) {
					pitr.Spec.PointInTimeRecoveryStorage = v1alpha1.PointInTimeRecoveryStorage{}
				},
				true,
			),
		)
	})
})
