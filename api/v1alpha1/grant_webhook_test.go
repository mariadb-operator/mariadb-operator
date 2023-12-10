package v1alpha1

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Grant webhook", func() {
	Context("When updating a Grant", Ordered, func() {
		key := types.NamespacedName{
			Name:      "grant-mariadb-webhook",
			Namespace: testNamespace,
		}
		BeforeAll(func() {
			grant := Grant{
				ObjectMeta: metav1.ObjectMeta{
					Name:      key.Name,
					Namespace: key.Namespace,
				},
				Spec: GrantSpec{
					MariaDBRef: MariaDBRef{
						ObjectReference: corev1.ObjectReference{
							Name: "mariadb-webhook",
						},
						WaitForIt: true,
					},
					Privileges: []string{
						"SELECT",
					},
					Database:    "foo",
					Table:       "foo",
					Username:    "foo",
					GrantOption: false,
				},
			}
			Expect(k8sClient.Create(testCtx, &grant)).To(Succeed())
		})

		DescribeTable(
			"Should validate",
			func(patchFn func(grant *Grant), wantErr bool) {
				var grant Grant
				Expect(k8sClient.Get(testCtx, key, &grant)).To(Succeed())

				patch := client.MergeFrom(grant.DeepCopy())
				patchFn(&grant)

				err := k8sClient.Patch(testCtx, &grant, patch)
				if wantErr {
					Expect(err).To(HaveOccurred())
				} else {
					Expect(err).ToNot(HaveOccurred())
				}
			},
			Entry(
				"Updating MariaDBRef",
				func(grant *Grant) {
					grant.Spec.MariaDBRef.Name = "another-mariadb"
				},
				true,
			),
			Entry(
				"Updating Privileges",
				func(grant *Grant) {
					grant.Spec.Privileges = []string{
						"SELECT",
						"UPDATE",
					}
				},
				true,
			),
			Entry(
				"Updating Database",
				func(grant *Grant) {
					grant.Spec.Database = "bar"
				},
				true,
			),
			Entry(
				"Updating Table",
				func(grant *Grant) {
					grant.Spec.Table = "bar"
				},
				true,
			),
			Entry(
				"Updating Username",
				func(grant *Grant) {
					grant.Spec.Username = "bar"
				},
				true,
			),
			Entry(
				"Updating GrantOption",
				func(grant *Grant) {
					grant.Spec.GrantOption = true
				},
				true,
			),
		)
	})
})
