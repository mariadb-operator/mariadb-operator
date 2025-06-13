package controller

// import (
// 	. "github.com/onsi/ginkgo/v2"
// 	. "github.com/onsi/gomega"
// )

// var _ = Describe("PhysicalBackup Controller", func() {
// 	Context("When reconciling a resource", func() {
// 		const resourceName = "test-resource"

// 		ctx := context.Background()

// 		typeNamespacedName := types.NamespacedName{
// 			Name:      resourceName,
// 			Namespace: "default", // TODO(user):Modify as needed
// 		}
// 		physicalbackup := &mariadbv1alpha1.PhysicalBackup{}

// 		BeforeEach(func() {
// 			By("creating the custom resource for the Kind PhysicalBackup")
// 			err := k8sClient.Get(ctx, typeNamespacedName, physicalbackup)
// 			if err != nil && errors.IsNotFound(err) {
// 				resource := &mariadbv1alpha1.PhysicalBackup{
// 					ObjectMeta: metav1.ObjectMeta{
// 						Name:      resourceName,
// 						Namespace: "default",
// 					},
// 				}
// 				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
// 			}
// 		})

// 		AfterEach(func() {
// 			resource := &mariadbv1alpha1.PhysicalBackup{}
// 			err := k8sClient.Get(ctx, typeNamespacedName, resource)
// 			Expect(err).NotTo(HaveOccurred())

// 			By("Cleanup the specific resource instance PhysicalBackup")
// 			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
// 		})
// 		It("should successfully reconcile the resource", func() {
// 			By("Reconciling the created resource")
// 			controllerReconciler := &PhysicalBackupReconciler{
// 				Client: k8sClient,
// 				Scheme: k8sClient.Scheme(),
// 			}

// 			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
// 				NamespacedName: typeNamespacedName,
// 			})
// 			Expect(err).NotTo(HaveOccurred())
// 			// TODO(user): Add more specific assertions depending on your controller's reconciliation logic.
// 			// Example: If you expect a certain status condition after reconciliation, verify it here.
// 		})
// 	})
// })
