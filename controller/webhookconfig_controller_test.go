/*
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

package controller

import (
	"time"

	"github.com/mariadb-operator/mariadb-operator/pkg/metadata"
	"github.com/mariadb-operator/mariadb-operator/pkg/pki"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	admissionregistration "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("WebhookConfig", func() {
	Context("When creating a MutatingWebhookConfiguration", func() {
		It("Should reconcile", func() {
			key := types.NamespacedName{
				Name:      "test-mutating",
				Namespace: testNamespace,
			}
			mutatingConfig := admissionregistration.MutatingWebhookConfiguration{
				ObjectMeta: metav1.ObjectMeta{
					Name:      key.Name,
					Namespace: key.Namespace,
					Annotations: map[string]string{
						metadata.WebhookConfigAnnotation: "",
					},
				},
				Webhooks: []admissionregistration.MutatingWebhook{
					{
						Name:                    "mmariadb.kb.io",
						AdmissionReviewVersions: []string{"v1"},
						ClientConfig: admissionregistration.WebhookClientConfig{
							Service: &admissionregistration.ServiceReference{
								Name:      "test",
								Namespace: "test",
								Path: func() *string {
									path := "/mutate"
									return &path
								}(),
							},
						},
						SideEffects: func() *admissionregistration.SideEffectClass {
							sideEffects := admissionregistration.SideEffectClassNone
							return &sideEffects
						}(),
					},
				},
			}
			By("Expecting to create a MutatingWebhookConfiguration")
			Expect(k8sClient.Create(testCtx, &mutatingConfig)).To(Succeed())

			By("Expecting to eventually inject CA bundle")
			Eventually(func() bool {
				Expect(k8sClient.Get(testCtx, key, &mutatingConfig)).To(Succeed())
				for _, w := range mutatingConfig.Webhooks {
					if w.ClientConfig.CABundle == nil {
						return false
					}
					if w.ClientConfig.Service.Name != testWebhookServiceKey.Name {
						return false
					}
					if w.ClientConfig.Service.Namespace != testWebhookServiceKey.Namespace {
						return false
					}
				}
				return true
			}, testTimeout, testInterval).Should(BeTrue())

			var caSecret corev1.Secret
			Expect("Expecting to create a CA Secret")
			Eventually(func() bool {
				return k8sClient.Get(testCtx, testCASecretKey, &caSecret) == nil
			}, testTimeout, testInterval).Should(BeTrue())

			var certSecret corev1.Secret
			Expect("Expecting to create a certificate Secret")
			Eventually(func() bool {
				return k8sClient.Get(testCtx, testCertSecretKey, &certSecret) == nil
			}, testTimeout, testInterval).Should(BeTrue())

			By("Expecting to get CA KeyPair")
			caKeyPair, err := pki.KeyPairFromTLSSecret(&caSecret)
			Expect(err).To(BeNil())
			Expect(caKeyPair).NotTo(BeNil())

			By("Expecting to get certificate KeyPair")
			certKeyPair, err := pki.KeyPairFromTLSSecret(&certSecret)
			Expect(err).To(BeNil())
			Expect(certKeyPair).NotTo(BeNil())

			By("Expecting certificate to be valid")
			dnsNames := pki.ServiceDNSNames(testWebhookServiceKey)
			valid, err := pki.ValidCert(caKeyPair, certKeyPair, dnsNames.CommonName, time.Now())
			Expect(err).To(BeNil())
			Expect(valid).To(BeTrue())

			By("Deleting Secret")
			Expect(k8sClient.Delete(testCtx, &caSecret)).To(Succeed())
			Expect(k8sClient.Delete(testCtx, &certSecret)).To(Succeed())
		})
	})

	Context("When creating a ValidatingWebhookConfiguration", func() {
		It("Should reconcile", func() {
			key := types.NamespacedName{
				Name:      "test-validating",
				Namespace: testNamespace,
			}
			validatingConfig := admissionregistration.ValidatingWebhookConfiguration{
				ObjectMeta: metav1.ObjectMeta{
					Name:      key.Name,
					Namespace: key.Namespace,
					Annotations: map[string]string{
						metadata.WebhookConfigAnnotation: "",
					},
				},
				Webhooks: []admissionregistration.ValidatingWebhook{
					{
						Name:                    "vbackup.kb.io",
						AdmissionReviewVersions: []string{"v1"},
						ClientConfig: admissionregistration.WebhookClientConfig{
							Service: &admissionregistration.ServiceReference{
								Name:      "test",
								Namespace: "test",
								Path: func() *string {
									path := "/validate"
									return &path
								}(),
							},
						},
						SideEffects: func() *admissionregistration.SideEffectClass {
							sideEffects := admissionregistration.SideEffectClassNone
							return &sideEffects
						}(),
					},
					{
						Name:                    "vconnection.kb.io",
						AdmissionReviewVersions: []string{"v1"},
						ClientConfig: admissionregistration.WebhookClientConfig{
							Service: &admissionregistration.ServiceReference{
								Name:      "test",
								Namespace: "test",
								Path: func() *string {
									path := "/validate"
									return &path
								}(),
							},
						},
						SideEffects: func() *admissionregistration.SideEffectClass {
							sideEffects := admissionregistration.SideEffectClassNone
							return &sideEffects
						}(),
					},
					{
						Name:                    "vdatabase.kb.io",
						AdmissionReviewVersions: []string{"v1"},
						ClientConfig: admissionregistration.WebhookClientConfig{
							Service: &admissionregistration.ServiceReference{
								Name:      "test",
								Namespace: "test",
								Path: func() *string {
									path := "/validate"
									return &path
								}(),
							},
						},
						SideEffects: func() *admissionregistration.SideEffectClass {
							sideEffects := admissionregistration.SideEffectClassNone
							return &sideEffects
						}(),
					},
				},
			}
			By("Expecting to create a ValidatingWebhookConfiguration")
			Expect(k8sClient.Create(testCtx, &validatingConfig)).To(Succeed())

			By("Expecting to eventually inject CA bundle")
			Eventually(func() bool {
				Expect(k8sClient.Get(testCtx, key, &validatingConfig)).To(Succeed())
				for _, w := range validatingConfig.Webhooks {
					if w.ClientConfig.CABundle == nil {
						return false
					}
					if w.ClientConfig.Service.Name != testWebhookServiceKey.Name {
						return false
					}
					if w.ClientConfig.Service.Namespace != testWebhookServiceKey.Namespace {
						return false
					}
				}
				return true
			}, testTimeout, testInterval).Should(BeTrue())

			var caSecret corev1.Secret
			Expect("Expecting to create a CA Secret")
			Eventually(func() bool {
				return k8sClient.Get(testCtx, testCASecretKey, &caSecret) == nil
			}, testTimeout, testInterval).Should(BeTrue())

			var certSecret corev1.Secret
			Expect("Expecting to create a certificate Secret")
			Eventually(func() bool {
				return k8sClient.Get(testCtx, testCertSecretKey, &certSecret) == nil
			}, testTimeout, testInterval).Should(BeTrue())

			By("Expecting to get CA KeyPair")
			caKeyPair, err := pki.KeyPairFromTLSSecret(&caSecret)
			Expect(err).To(BeNil())
			Expect(caKeyPair).NotTo(BeNil())

			By("Expecting to get certificate KeyPair")
			certKeyPair, err := pki.KeyPairFromTLSSecret(&certSecret)
			Expect(err).To(BeNil())
			Expect(certKeyPair).NotTo(BeNil())

			By("Expecting certificate to be valid")
			dnsNames := pki.ServiceDNSNames(testWebhookServiceKey)
			valid, err := pki.ValidCert(caKeyPair, certKeyPair, dnsNames.CommonName, time.Now())
			Expect(err).To(BeNil())
			Expect(valid).To(BeTrue())

			By("Deleting Secret")
			Expect(k8sClient.Delete(testCtx, &caSecret)).To(Succeed())
			Expect(k8sClient.Delete(testCtx, &certSecret)).To(Succeed())
		})
	})
})
