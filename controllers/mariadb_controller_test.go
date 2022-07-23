package controllers

import (
	"fmt"
	"time"

	databasev1alpha1 "github.com/mmontes11/mariadb-operator/api/v1alpha1"
	"github.com/mmontes11/mariadb-operator/pkg/portforwarder"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sethvargo/go-password/password"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const (
	timeout  = time.Second * 30
	interval = time.Second * 1
)

var (
	defaultNamespace       = "default"
	defaultStorageClass    = "standard"
	mariaDbName            = "mariadb-test"
	rootPasswordSecretName = "root-test"
	rootPasswordSecretKey  = "passsword"
)

var _ = Describe("MariaDB controller", func() {
	var secret v1.Secret
	var mariaDbKey types.NamespacedName
	var mariaDb databasev1alpha1.MariaDB

	BeforeEach(func() {
		By("Creating root password Secret")

		secretKey := types.NamespacedName{
			Name:      rootPasswordSecretName,
			Namespace: defaultNamespace,
		}
		password, err := password.Generate(16, 4, 0, false, false)
		Expect(err).NotTo(HaveOccurred())
		secret = v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretKey.Name,
				Namespace: secretKey.Namespace,
			},
			Data: map[string][]byte{
				rootPasswordSecretKey: []byte(password),
			},
		}
		Expect(k8sClient.Create(ctx, &secret)).To(Succeed())

		By("Creating MariaDB")

		mariaDbKey = types.NamespacedName{
			Name:      mariaDbName,
			Namespace: defaultNamespace,
		}
		storageSize, err := resource.ParseQuantity("100Mi")
		Expect(err).ToNot(HaveOccurred())
		mariaDb = databasev1alpha1.MariaDB{
			ObjectMeta: metav1.ObjectMeta{
				Name:      mariaDbKey.Name,
				Namespace: mariaDbKey.Namespace,
			},
			Spec: databasev1alpha1.MariaDBSpec{
				RootPasswordSecretKeyRef: corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: secretKey.Name,
					},
					Key: rootPasswordSecretKey,
				},
				Image: databasev1alpha1.Image{
					Repository: "mariadb",
					Tag:        "10.7.4",
				},
				Storage: databasev1alpha1.Storage{
					ClassName: defaultStorageClass,
					Size:      storageSize,
				},
			},
		}
		Expect(k8sClient.Create(ctx, &mariaDb)).To(Succeed())
	})

	AfterEach(func() {
		By("Tearing down initial resources")
		Expect(k8sClient.Delete(ctx, &mariaDb)).To(Succeed())
		Expect(k8sClient.Delete(ctx, &secret)).To(Succeed())
	})

	Context("When creating a MariaDB", func() {
		It("Should reconcile", func() {
			var mariaDb databasev1alpha1.MariaDB

			By("Expecting to be ready eventually")
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, mariaDbKey, &mariaDb); err != nil {
					return false
				}
				return mariaDb.IsReady()
			}, timeout, interval).Should(BeTrue())

			By("Expecting to have ready condition")
			hasReadyCondition := meta.IsStatusConditionTrue(mariaDb.Status.Conditions, databasev1alpha1.ConditionTypeReady)
			Expect(hasReadyCondition).To(BeTrue())

			By("Expecting to have spec provided by user and defaults")
			Expect(mariaDb.Spec.Image.String()).To(Equal("mariadb:10.7.4"))
			Expect(mariaDb.Spec.Port).To(BeEquivalentTo(3306))
			Expect(mariaDb.Spec.Storage.ClassName).To(Equal("standard"))
			Expect(mariaDb.Spec.Storage.AccessModes).To(ConsistOf(corev1.ReadWriteOnce))

			var sts appsv1.StatefulSet
			By("Expecting to create a StatefulSet eventually")
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, mariaDbKey, &sts); err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())
			Expect(sts).ToNot(BeNil())

			By("Expecting to create a Service")
			var svc corev1.Service
			Expect(k8sClient.Get(ctx, mariaDbKey, &svc)).To(Succeed())
			Expect(svc).ToNot(BeNil())
		})

		It("Should bootstrap from backup", func() {
			By("Creating a port forward to MariaDB")
			pod := fmt.Sprintf("%s-0", mariaDbKey.Name)
			pf, err := portforwarder.New().
				WithPod(pod).
				WithNamespace(mariaDbKey.Namespace).
				WithPorts("3306").
				WithOutputWriter(GinkgoWriter).
				WithErrorWriter(GinkgoWriter).
				Build()
			Expect(err).ToNot(HaveOccurred())

			go func() {
				if err := pf.Run(ctx); err != nil {
					Fail(fmt.Sprintf("failed creating port forward to Pod '%s': %v", pod, err))
				}
			}()
		})
	})
})
