package statefulset

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("StatefulSetValidPodName", func() {
	DescribeTable("validates pod names",
		func(meta metav1.ObjectMeta, replicas int, podName string, wantErr bool) {
			err := ValidPodName(meta, replicas, podName)
			if wantErr {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
			}
		},
		Entry("empty", metav1.ObjectMeta{Name: ""}, 0, "", true),
		Entry("negative replicas", metav1.ObjectMeta{Name: ""}, -1, "", true),
		Entry("no index no prefix", metav1.ObjectMeta{Name: "mariadb-galera"}, 3, "foo", true),
		Entry("no index", metav1.ObjectMeta{Name: "mariadb-galera"}, 3, "mariadb-galera", true),
		Entry("invalid index", metav1.ObjectMeta{Name: "mariadb-galera"}, 3, "mariadb-galera-5", true),
		Entry("no prefix", metav1.ObjectMeta{Name: "mariadb-galera"}, 3, "foo-0", true),
		Entry("valid", metav1.ObjectMeta{Name: "mariadb-galera"}, 3, "mariadb-galera-0", false),
	)
})
