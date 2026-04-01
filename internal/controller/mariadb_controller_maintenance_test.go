package controller

import (
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/refresolver"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/sql"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("MariaDB Maintenance",  Ordered, Label("basic"), func() {
	key := types.NamespacedName{
		Name:      "mariadb-repl",
		Namespace: testNamespace,
	}
	mdb := &mariadbv1alpha1.MariaDB{
		ObjectMeta: metav1.ObjectMeta{
			Name:      key.Name,
			Namespace: key.Namespace,
		},
		Spec: mariadbv1alpha1.MariaDBSpec{
			Username: &testUser,
			PasswordSecretKeyRef: &mariadbv1alpha1.GeneratedSecretKeyRef{
				SecretKeySelector: mariadbv1alpha1.SecretKeySelector{
					LocalObjectReference: mariadbv1alpha1.LocalObjectReference{
						Name: testPwdKey.Name,
					},
					Key: testPwdSecretKey,
				},
			},
			Maintenance: &mariadbv1alpha1.MariaDBMaintenance{
				Enabled: false,
				Cordoning: mariadbv1alpha1.Cordoning{
					Cordon: false,
				},
				DrainConnections: false,
				ReadOnly:         false,
			},
			Replication: &mariadbv1alpha1.Replication{
				ReplicationSpec: mariadbv1alpha1.ReplicationSpec{
					Primary: mariadbv1alpha1.PrimaryReplication{
						PodIndex:     ptr.To(0),
						AutoFailover: ptr.To(true),
					},
				},
				Enabled: true,
			},
			Replicas: 3,
			Storage: mariadbv1alpha1.Storage{
				Size: ptr.To(resource.MustParse("300Mi")),
			},
			Service: &mariadbv1alpha1.ServiceTemplate{
				Type: corev1.ServiceTypeLoadBalancer,
				Metadata: &mariadbv1alpha1.Metadata{
					Annotations: map[string]string{
						"metallb.universe.tf/loadBalancerIPs": testCidrPrefix + ".0.120",
					},
				},
			},
			PrimaryService: &mariadbv1alpha1.ServiceTemplate{
				Type: corev1.ServiceTypeLoadBalancer,
				Metadata: &mariadbv1alpha1.Metadata{
					Annotations: map[string]string{
						"metallb.universe.tf/loadBalancerIPs": testCidrPrefix + ".0.130",
					},
				},
			},
			SecondaryService: &mariadbv1alpha1.ServiceTemplate{
				Type: corev1.ServiceTypeLoadBalancer,
				Metadata: &mariadbv1alpha1.Metadata{
					Annotations: map[string]string{
						"metallb.universe.tf/loadBalancerIPs": testCidrPrefix + ".0.131",
					},
				},
			},
		},
	}

	BeforeAll(func() {
		By("Creating MariaDB")
		Expect(k8sClient.Create(testCtx, mdb)).To(Succeed())
		DeferCleanup(func() {
			deleteMariadb(key, false)
		})

		By("Expecting MariaDB to be ready eventually")
		Eventually(func() bool {
			if err := k8sClient.Get(testCtx, key, mdb); err != nil {
				return false
			}
			return mdb.IsReady()
		}, testHighTimeout, testInterval).Should(BeTrue())
	})

	It("should be able to be set in readOnly", func() {
		By("Set MariaDB In ReadOnly")
		Eventually(func() bool {
			if err := k8sClient.Get(testCtx, key, mdb); err != nil {
				return false
			}
			mdb.Spec.Maintenance.Enabled = true
			mdb.Spec.Maintenance.ReadOnly = true

			return k8sClient.Update(testCtx, mdb) == nil
		}, testTimeout, testInterval).Should(BeTrue())

		By("Expecting MariaDB to be accessible from headless service")
		sqlClient, err := sql.NewInternalClientWithPodIndex(testCtx, mdb, refresolver.New(k8sClient), 0)
		Expect(err).To(Succeed())
		defer sqlClient.Close()

		By("Expecting MariaDB to eventually be read_only")
		expectMariadbFn(testCtx, k8sClient, key, func(mdb *mariadbv1alpha1.MariaDB) bool {
			isReadOnly, err := sqlClient.GetReadOnly(testCtx)
			if err != nil {
				return false
			}

			return isReadOnly
		})

		By("Removing ReadOnly")
		Eventually(func() bool {
			if err := k8sClient.Get(testCtx, key, mdb); err != nil {
				return false
			}
			mdb.Spec.Maintenance.ReadOnly = false

			return k8sClient.Update(testCtx, mdb) == nil
		}, testTimeout, testInterval).Should(BeTrue())

		By("Expecting MariaDB to eventually not be read_only")
		expectMariadbFn(testCtx, k8sClient, key, func(mdb *mariadbv1alpha1.MariaDB) bool {
			isReadOnly, err := sqlClient.GetReadOnly(testCtx)
			if err != nil {
				return false
			}

			return !isReadOnly
		})
	})

	It("should be cordonable", func() {
		By("Cordoning MariaDB")
		Eventually(func() bool {
			if err := k8sClient.Get(testCtx, key, mdb); err != nil {
				return false
			}
			mdb.Spec.Maintenance.Cordon = true

			return k8sClient.Update(testCtx, mdb) == nil
		}, testTimeout, testInterval).Should(BeTrue())

		By("Expecting MariaDB to eventually be cordoned")
		expectMariadbFn(testCtx, k8sClient, key, func(mdb *mariadbv1alpha1.MariaDB) bool {
			return mdb.IsCordoned()
		})

		By("Expecting Default Service To Have Cordon Labels")
		Eventually(func() bool {
			var svc corev1.Service
			if err := k8sClient.Get(testCtx, client.ObjectKeyFromObject(mdb), &svc); err != nil {
				return false
			}

			selector := svc.Spec.Selector

			_, ok := selector["k8s.mariadb.com/cordon"]

			return ok
		}, testTimeout, testInterval).Should(BeTrue())

		By("Expecting Primary Service To Have Cordon Labels")
		Eventually(func() bool {
			var svc corev1.Service
			if err := k8sClient.Get(testCtx, mdb.PrimaryServiceKey(), &svc); err != nil {
				return false
			}

			selector := svc.Spec.Selector

			_, ok := selector["k8s.mariadb.com/cordon"]

			return ok
		}, testTimeout, testInterval).Should(BeTrue())

		By("Expecting Secondary Service To Have Cordon Labels")
		Eventually(func() bool {
			var svc discoveryv1.EndpointSlice
			if err := k8sClient.Get(testCtx, mdb.SecondaryServiceKey(), &svc); err != nil {
				return false
			}

			return len(svc.Endpoints) == 0
		}, testTimeout, testInterval).Should(BeTrue())
	})
})
