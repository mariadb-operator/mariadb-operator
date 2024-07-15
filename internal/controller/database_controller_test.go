package controller

import (
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

var _ = Describe("Database", func() {
	BeforeEach(func() {
		By("Waiting for MariaDB to be ready")
		expectMariadbReady(testCtx, k8sClient, testMdbkey)
	})

	It("should reconcile", func() {
		By("Creating a Database")
		databaseKey := types.NamespacedName{
			Name:      "database-create-test",
			Namespace: testNamespace,
		}
		database := mariadbv1alpha1.Database{
			ObjectMeta: metav1.ObjectMeta{
				Name:      databaseKey.Name,
				Namespace: databaseKey.Namespace,
			},
			Spec: mariadbv1alpha1.DatabaseSpec{
				MariaDBRef: mariadbv1alpha1.MariaDBRef{
					ObjectReference: corev1.ObjectReference{
						Name: testMdbkey.Name,
					},
					WaitForIt: true,
				},
				CharacterSet: "utf8",
				Collate:      "utf8_general_ci",
			},
		}
		Expect(k8sClient.Create(testCtx, &database)).To(Succeed())
		DeferCleanup(func() {
			Expect(k8sClient.Delete(testCtx, &database)).To(Succeed())
		})

		By("Expecting Database to be ready eventually")
		Eventually(func() bool {
			if err := k8sClient.Get(testCtx, databaseKey, &database); err != nil {
				return false
			}
			return database.IsReady()
		}, testTimeout, testInterval).Should(BeTrue())

		By("Expecting Database to eventually have finalizer")
		Eventually(func() bool {
			if err := k8sClient.Get(testCtx, databaseKey, &database); err != nil {
				return false
			}
			return controllerutil.ContainsFinalizer(&database, databaseFinalizerName)
		}, testTimeout, testInterval).Should(BeTrue())
	})
})
