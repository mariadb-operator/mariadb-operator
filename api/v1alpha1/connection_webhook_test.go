/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Connection webhook", func() {
	Context("When updating a Connection", func() {
		It("Should validate", func() {
			By("Creating Connection")
			key := types.NamespacedName{
				Name:      "conn-update",
				Namespace: testNamespace,
			}
			conn := Connection{
				ObjectMeta: v1.ObjectMeta{
					Name:      key.Name,
					Namespace: key.Namespace,
				},
				Spec: ConnectionSpec{
					ConnectionTemplate: ConnectionTemplate{
						SecretName: func() *string { t := "test"; return &t }(),
						SecretTemplate: &SecretTemplate{
							Labels: map[string]string{
								"foo": "bar",
							},
						},
						HealthCheck: &HealthCheck{
							Interval:      &metav1.Duration{Duration: 1 * time.Second},
							RetryInterval: &metav1.Duration{Duration: 1 * time.Second},
						},
					},
					MariaDBRef: MariaDBRef{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "mariadb-webhook",
						},
						WaitForIt: true,
					},
					Username: "test",
					PasswordSecretKeyRef: corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "test",
						},
						Key: "dsn",
					},
					Database: func() *string { t := "test"; return &t }(),
				},
			}
			Expect(k8sClient.Create(testCtx, &conn)).To(Succeed())

			// TODO: migrate to Ginkgo v2 and use Ginkgo table tests
			// https://github.com/mariadb-operator/mariadb-operator/issues/3
			tt := []struct {
				by      string
				patchFn func(conn *Connection)
				wantErr bool
			}{
				{
					by: "Updating MariaDBRef",
					patchFn: func(conn *Connection) {
						conn.Spec.MariaDBRef.Name = "foo"
					},
					wantErr: true,
				},
				{
					by: "Updating Username",
					patchFn: func(conn *Connection) {
						conn.Spec.Username = "foo"
					},
					wantErr: true,
				},
				{
					by: "Updating PasswordSecretKeyRef",
					patchFn: func(conn *Connection) {
						conn.Spec.PasswordSecretKeyRef.Key = "foo"
					},
					wantErr: true,
				},
				{
					by: "Updating Database",
					patchFn: func(conn *Connection) {
						conn.Spec.Database = func() *string { t := "foo"; return &t }()
					},
					wantErr: true,
				},
				{
					by: "Updating SecretName",
					patchFn: func(conn *Connection) {
						conn.Spec.SecretName = func() *string { s := "foo"; return &s }()
					},
					wantErr: true,
				},
				{
					by: "Updating SecretTemplate",
					patchFn: func(conn *Connection) {
						conn.Spec.SecretTemplate.Labels = map[string]string{
							"foo": "foo",
						}
					},
					wantErr: true,
				},
				{
					by: "Updating HealthCheck",
					patchFn: func(conn *Connection) {
						conn.Spec.HealthCheck.Interval = &metav1.Duration{Duration: 3 * time.Second}
						conn.Spec.HealthCheck.RetryInterval = &metav1.Duration{Duration: 3 * time.Second}
					},
					wantErr: false,
				},
			}

			for _, t := range tt {
				By(t.by)
				Expect(k8sClient.Get(testCtx, key, &conn)).To(Succeed())

				patch := client.MergeFrom(conn.DeepCopy())
				t.patchFn(&conn)

				err := k8sClient.Patch(testCtx, &conn, patch)
				if t.wantErr {
					Expect(err).To(HaveOccurred())
				} else {
					Expect(err).ToNot(HaveOccurred())
				}
			}
		})
	})
})
