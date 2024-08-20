package v1alpha1

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Database webhook", func() {
	Context("When creating a Database", func() {
		key := types.NamespacedName{
			Name:      "database-create-webhook",
			Namespace: testNamespace,
		}
		DescribeTable(
			"Should validate",
			func(database *Database, wantErr bool) {
				err := k8sClient.Create(testCtx, database)
				if wantErr {
					Expect(err).To(HaveOccurred())
				} else {
					Expect(err).ToNot(HaveOccurred())
				}
			},
			Entry(
				"Valid cleanupPolicy",
				&Database{
					ObjectMeta: metav1.ObjectMeta{
						Name:      key.Name,
						Namespace: key.Namespace,
					},
					Spec: DatabaseSpec{
						SQLTemplate: SQLTemplate{
							CleanupPolicy: ptr.To(CleanupPolicyDelete),
						},
						MariaDBRef: MariaDBRef{
							ObjectReference: corev1.ObjectReference{
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
				&Database{
					ObjectMeta: metav1.ObjectMeta{
						Name:      key.Name,
						Namespace: key.Namespace,
					},
					Spec: DatabaseSpec{
						SQLTemplate: SQLTemplate{
							CleanupPolicy: ptr.To(CleanupPolicy("")),
						},
						MariaDBRef: MariaDBRef{
							ObjectReference: corev1.ObjectReference{
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

	Context("When updating a Database", Ordered, func() {
		key := types.NamespacedName{
			Name:      "database-update-webhook",
			Namespace: testNamespace,
		}
		BeforeAll(func() {
			database := Database{
				ObjectMeta: metav1.ObjectMeta{
					Name:      key.Name,
					Namespace: key.Namespace,
				},
				Spec: DatabaseSpec{
					MariaDBRef: MariaDBRef{
						ObjectReference: corev1.ObjectReference{
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
			func(patchFn func(db *Database), wantErr bool) {
				var db Database
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
				func(db *Database) {
					db.Spec.MariaDBRef.Name = "another-mariadb"
				},
				true,
			),
			Entry(
				"Updating CharacterSet",
				func(db *Database) {
					db.Spec.CharacterSet = "utf16"
				},
				true,
			),
			Entry(
				"Updating Collate",
				func(db *Database) {
					db.Spec.Collate = "latin2_general_ci"
				},
				true,
			),
			Entry(
				"Updating to valid CleanupPolicy",
				func(database *Database) {
					database.Spec.CleanupPolicy = ptr.To(CleanupPolicySkip)
				},
				false,
			),
			Entry(
				"Updating to invalid CleanupPolicy",
				func(database *Database) {
					database.Spec.CleanupPolicy = ptr.To(CleanupPolicy(""))
				},
				true,
			),
		)
	})
})
