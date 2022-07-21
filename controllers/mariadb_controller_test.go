package controllers

import (
	"time"

	databasev1alpha1 "github.com/mmontes11/mariadb-operator/api/v1alpha1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sethvargo/go-password/password"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const (
	timeout  = time.Second * 30
	interval = time.Second * 1
)

var (
	defaultNamespace      = "default"
	rootPasswordSecretKey = "passsword"
)

var _ = Describe("MariaDB controller", func() {

	var mariaDbKey types.NamespacedName

	BeforeEach(func() {
		password, err := password.Generate(16, 4, 0, false, false)
		Expect(err).NotTo(HaveOccurred())

		secretKey := types.NamespacedName{
			Name:      "root",
			Namespace: defaultNamespace,
		}
		secret := &v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretKey.Name,
				Namespace: secretKey.Namespace,
			},
			Data: map[string][]byte{
				rootPasswordSecretKey: []byte(password),
			},
		}
		Expect(k8sClient.Create(ctx, secret)).To(Succeed())

		storageSize, err := resource.ParseQuantity("100Mi")
		Expect(err).ToNot(HaveOccurred())

		mariaDbKey = types.NamespacedName{
			Name:      "mariadb",
			Namespace: defaultNamespace,
		}
		mariaDb := &databasev1alpha1.MariaDB{
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
				Port: 3306,
				Storage: databasev1alpha1.Storage{
					ClassName: "default",
					Size:      storageSize,
				},
			},
		}
		Expect(k8sClient.Create(ctx, mariaDb)).To(Succeed())
	})

	Context("When creating a MariaDB", func() {
		It("Should eventually be ready", func() {
			var mariaDb databasev1alpha1.MariaDB

			Eventually(func() bool {
				if err := k8sClient.Get(ctx, mariaDbKey, &mariaDb); err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())
		})
	})
})
