package builder

import (
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/discovery"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

var _ = Describe("BuildContainerSecurityContext", func() {
	It("should build the container SecurityContext", func() {
		builder := newDefaultTestBuilder()

		sc, err := builder.buildContainerSecurityContext(&mariadbv1alpha1.SecurityContext{
			RunAsUser: ptr.To(mysqlUser),
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(sc).NotTo(BeNil())

		resource := &metav1.APIResourceList{
			GroupVersion: "security.openshift.io/v1",
			APIResources: []metav1.APIResource{
				{
					Name: "securitycontextconstraints",
				},
			},
		}
		fakeDiscovery, err := discovery.NewFakeDiscovery(resource)
		Expect(err).NotTo(HaveOccurred())
		builder = newTestBuilder(fakeDiscovery)

		sc, err = builder.buildContainerSecurityContext(&mariadbv1alpha1.SecurityContext{
			RunAsUser: ptr.To(mysqlUser),
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(sc).To(BeNil())
	})
})

var _ = Describe("BuildPodSecurityContext", func() {
	It("should build the pod SecurityContext", func() {
		builder := newDefaultTestBuilder()

		sc, err := builder.buildPodSecurityContext(&mariadbv1alpha1.PodSecurityContext{
			RunAsUser: ptr.To(mysqlUser),
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(sc).NotTo(BeNil())

		resource := &metav1.APIResourceList{
			GroupVersion: "security.openshift.io/v1",
			APIResources: []metav1.APIResource{
				{
					Name: "securitycontextconstraints",
				},
			},
		}
		fakeDiscovery, err := discovery.NewFakeDiscovery(resource)
		Expect(err).NotTo(HaveOccurred())
		builder = newTestBuilder(fakeDiscovery)

		sc, err = builder.buildPodSecurityContext(&mariadbv1alpha1.PodSecurityContext{
			RunAsUser: ptr.To(mysqlUser),
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(sc).To(BeNil())
	})
})

var _ = Describe("BuildPodSecurityContextWithUserGroup", func() {
	It("should build the pod SecurityContext with user and group", func() {
		builder := newDefaultTestBuilder()

		sc, err := builder.buildPodSecurityContextWithUserGroup(&mariadbv1alpha1.PodSecurityContext{
			RunAsUser: ptr.To(mysqlUser),
		}, mysqlUser, mysqlGroup)
		Expect(err).NotTo(HaveOccurred())
		Expect(sc).NotTo(BeNil())

		sc, err = builder.buildPodSecurityContextWithUserGroup(nil, mysqlUser, mysqlGroup)
		Expect(err).NotTo(HaveOccurred())
		Expect(sc).NotTo(BeNil())
		Expect(ptr.Deref(sc.RunAsUser, 0)).To(Equal(mysqlUser))
		Expect(ptr.Deref(sc.RunAsGroup, 0)).To(Equal(mysqlGroup))
		Expect(ptr.Deref(sc.FSGroup, 0)).To(Equal(mysqlGroup))

		resource := &metav1.APIResourceList{
			GroupVersion: "security.openshift.io/v1",
			APIResources: []metav1.APIResource{
				{
					Name: "securitycontextconstraints",
				},
			},
		}
		fakeDiscovery, err := discovery.NewFakeDiscovery(resource)
		Expect(err).NotTo(HaveOccurred())
		builder = newTestBuilder(fakeDiscovery)

		sc, err = builder.buildPodSecurityContextWithUserGroup(&mariadbv1alpha1.PodSecurityContext{
			RunAsUser: ptr.To(mysqlUser),
		}, mysqlUser, mysqlGroup)
		Expect(err).NotTo(HaveOccurred())
		Expect(sc).To(BeNil())
	})
})
