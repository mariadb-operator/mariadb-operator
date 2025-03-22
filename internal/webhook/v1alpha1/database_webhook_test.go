package v1alpha1

import (
	"github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("v1alpha1.Database webhook", func() {
	Context("When creating a v1alpha1.Database", func() {
		key := types.NamespacedName{
			Name:      "database-create-webhook",
			Namespace: testNamespace,
		}
		DescribeTable(
			"Should validate",
			func(database *v1alpha1.Database, wantErr bool) {
				err := k8sClient.Create(testCtx, database)
				if wantErr {
					Expect(err).To(HaveOccurred())
				} else {
					Expect(err).ToNot(HaveOccurred())
				}
			},
			Entry(
				"Valid cleanupPolicy",
				&v1alpha1.Database{
					ObjectMeta: metav1.ObjectMeta{
						Name:      key.Name,
						Namespace: key.Namespace,
					},
					Spec: v1alpha1.DatabaseSpec{
						SQLTemplate: v1alpha1.SQLTemplate{
							CleanupPolicy: ptr.To(v1alpha1.CleanupPolicyDelete),
						},
						MariaDBRef: v1alpha1.MariaDBRef{
							ObjectReference: v1alpha1.ObjectReference{
								Name: "mariadb-webhook",
							},
							WaitForIt: true,
						},
						CharacterSet: "utf8",
						Collate:      "utf8_general_ci",
					},
				},
				false,
			),
			Entry(
				"Invalid cleanupPolicy",
				&v1alpha1.Database{
					ObjectMeta: metav1.ObjectMeta{
						Name:      key.Name,
						Namespace: key.Namespace,
					},
					Spec: v1alpha1.DatabaseSpec{
						SQLTemplate: v1alpha1.SQLTemplate{
							CleanupPolicy: ptr.To(v1alpha1.CleanupPolicy("")),
						},
						MariaDBRef: v1alpha1.MariaDBRef{
							ObjectReference: v1alpha1.ObjectReference{
								Name: "mariadb-webhook",
							},
							WaitForIt: true,
						},
						CharacterSet: "utf8",
						Collate:      "utf8_general_ci",
					},
				},
				true,
			),
		)
	})

	Context("When updating a v1alpha1.Database", Ordered, func() {
		key := types.NamespacedName{
			Name:      "database-update-webhook",
			Namespace: testNamespace,
		}
		BeforeAll(func() {
			database := v1alpha1.Database{
				ObjectMeta: metav1.ObjectMeta{
					Name:      key.Name,
					Namespace: key.Namespace,
				},
				Spec: v1alpha1.DatabaseSpec{
					MariaDBRef: v1alpha1.MariaDBRef{
						ObjectReference: v1alpha1.ObjectReference{
							Name: "mariadb-webhook",
						},
						WaitForIt: true,
					},
					CharacterSet: "utf8",
					Collate:      "utf8_general_ci",
				},
			}
			Expect(k8sClient.Create(testCtx, &database)).To(Succeed())
		})

		DescribeTable(
			"Should validate",
			func(patchFn func(db *v1alpha1.Database), wantErr bool) {
				var db v1alpha1.Database
				Expect(k8sClient.Get(testCtx, key, &db)).To(Succeed())

				patch := client.MergeFrom(db.DeepCopy())
				patchFn(&db)

				err := k8sClient.Patch(testCtx, &db, patch)
				if wantErr {
					Expect(err).To(HaveOccurred())
				} else {
					Expect(err).ToNot(HaveOccurred())
				}
			},
			Entry(
				"Updating MariaDBRef",
				func(db *v1alpha1.Database) {
					db.Spec.MariaDBRef.Name = "another-mariadb"
				},
				true,
			),
			Entry(
				"Updating CharacterSet",
				func(db *v1alpha1.Database) {
					db.Spec.CharacterSet = "utf16"
				},
				true,
			),
			Entry(
				"Updating Collate",
				func(db *v1alpha1.Database) {
					db.Spec.Collate = "latin2_general_ci"
				},
				true,
			),
			Entry(
				"Updating to valid CleanupPolicy",
				func(database *v1alpha1.Database) {
					database.Spec.CleanupPolicy = ptr.To(v1alpha1.CleanupPolicySkip)
				},
				false,
			),
			Entry(
				"Updating to invalid CleanupPolicy",
				func(database *v1alpha1.Database) {
					database.Spec.CleanupPolicy = ptr.To(v1alpha1.CleanupPolicy(""))
				},
				true,
			),
		)
	})
})
