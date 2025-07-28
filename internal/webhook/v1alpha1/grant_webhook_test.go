package v1alpha1

import (
	"github.com/mariadb-operator/mariadb-operator/v25/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("v1alpha1.Grant webhook", func() {
	Context("When creating a v1alpha1.Grant", func() {
		key := types.NamespacedName{
			Name:      "grant-create-webhook",
			Namespace: testNamespace,
		}
		DescribeTable(
			"Should validate",
			func(grant *v1alpha1.Grant, wantErr bool) {
				err := k8sClient.Create(testCtx, grant)
				if wantErr {
					Expect(err).To(HaveOccurred())
				} else {
					Expect(err).ToNot(HaveOccurred())
				}
			},
			Entry(
				"Valid cleanupPolicy",
				&v1alpha1.Grant{
					ObjectMeta: metav1.ObjectMeta{
						Name:      key.Name,
						Namespace: key.Namespace,
					},
					Spec: v1alpha1.GrantSpec{
						SQLTemplate: v1alpha1.SQLTemplate{
							CleanupPolicy: ptr.To(v1alpha1.CleanupPolicyDelete),
						},
						MariaDBRef: v1alpha1.MariaDBRef{
							ObjectReference: v1alpha1.ObjectReference{
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
				},
				false,
			),
			Entry(
				"Invalid cleanupPolicy",
				&v1alpha1.Grant{
					ObjectMeta: metav1.ObjectMeta{
						Name:      key.Name,
						Namespace: key.Namespace,
					},
					Spec: v1alpha1.GrantSpec{
						SQLTemplate: v1alpha1.SQLTemplate{
							CleanupPolicy: ptr.To(v1alpha1.CleanupPolicy("")),
						},
						MariaDBRef: v1alpha1.MariaDBRef{
							ObjectReference: v1alpha1.ObjectReference{
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
				},
				true,
			),
		)
	})

	Context("When updating a v1alpha1.Grant", Ordered, func() {
		key := types.NamespacedName{
			Name:      "grant-update-webhook",
			Namespace: testNamespace,
		}
		BeforeAll(func() {
			grant := v1alpha1.Grant{
				ObjectMeta: metav1.ObjectMeta{
					Name:      key.Name,
					Namespace: key.Namespace,
				},
				Spec: v1alpha1.GrantSpec{
					MariaDBRef: v1alpha1.MariaDBRef{
						ObjectReference: v1alpha1.ObjectReference{
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
			func(patchFn func(grant *v1alpha1.Grant), wantErr bool) {
				var grant v1alpha1.Grant
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
				func(grant *v1alpha1.Grant) {
					grant.Spec.MariaDBRef.Name = "another-mariadb"
				},
				true,
			),
			Entry(
				"Updating Privileges",
				func(grant *v1alpha1.Grant) {
					grant.Spec.Privileges = []string{
						"SELECT",
						"UPDATE",
					}
				},
				false,
			),
			Entry(
				"Updating Database",
				func(grant *v1alpha1.Grant) {
					grant.Spec.Database = "bar"
				},
				true,
			),
			Entry(
				"Updating Table",
				func(grant *v1alpha1.Grant) {
					grant.Spec.Table = "bar"
				},
				true,
			),
			Entry(
				"Updating Username",
				func(grant *v1alpha1.Grant) {
					grant.Spec.Username = "bar"
				},
				true,
			),
			Entry(
				"Updating GrantOption",
				func(grant *v1alpha1.Grant) {
					grant.Spec.GrantOption = true
				},
				true,
			),
			Entry(
				"Updating to valid CleanupPolicy",
				func(grant *v1alpha1.Grant) {
					grant.Spec.CleanupPolicy = ptr.To(v1alpha1.CleanupPolicySkip)
				},
				false,
			),
			Entry(
				"Updating to invalid CleanupPolicy",
				func(grant *v1alpha1.Grant) {
					grant.Spec.CleanupPolicy = ptr.To(v1alpha1.CleanupPolicy(""))
				},
				true,
			),
		)
	})
})
