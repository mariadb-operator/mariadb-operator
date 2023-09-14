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

package controllers

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/pkg/annotation"
	"github.com/mariadb-operator/mariadb-operator/pkg/builder"
	"github.com/mariadb-operator/mariadb-operator/pkg/conditions"
	"github.com/mariadb-operator/mariadb-operator/pkg/controller/batch"
	"github.com/mariadb-operator/mariadb-operator/pkg/controller/configmap"
	"github.com/mariadb-operator/mariadb-operator/pkg/controller/endpoints"
	"github.com/mariadb-operator/mariadb-operator/pkg/controller/galera"
	"github.com/mariadb-operator/mariadb-operator/pkg/controller/rbac"
	"github.com/mariadb-operator/mariadb-operator/pkg/controller/replication"
	"github.com/mariadb-operator/mariadb-operator/pkg/controller/secret"
	"github.com/mariadb-operator/mariadb-operator/pkg/controller/service"
	"github.com/mariadb-operator/mariadb-operator/pkg/docker"
	"github.com/mariadb-operator/mariadb-operator/pkg/environment"
	"github.com/mariadb-operator/mariadb-operator/pkg/refresolver"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	//+kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var k8sClient client.Client
var testEnv *envtest.Environment
var testCtx context.Context
var testCancel context.CancelFunc
var testCidrPrefix string

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Controller Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	testCtx, testCancel = context.WithCancel(context.Background())
	useCluster := true

	var err error
	testCidrPrefix, err = docker.GetKindCidrPrefix()
	Expect(testCidrPrefix).NotTo(BeEmpty())
	Expect(err).NotTo(HaveOccurred())

	By("Bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,
		UseExistingCluster:    &useCluster,
	}

	cfg, err := testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	err = mariadbv1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	err = monitoringv1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	//+kubebuilder:scaffold:scheme

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	k8sManager, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme.Scheme,
	})
	Expect(err).ToNot(HaveOccurred())

	client := k8sManager.GetClient()
	scheme := k8sManager.GetScheme()
	galeraRecorder := k8sManager.GetEventRecorderFor("galera")
	replRecorder := k8sManager.GetEventRecorderFor("replication")

	env, err := environment.GetEnvironment(testCtx)
	Expect(err).ToNot(HaveOccurred())

	builder := builder.NewBuilder(scheme, env)
	refResolver := refresolver.New(client)

	conditionReady := conditions.NewReady()
	conditionComplete := conditions.NewComplete(client)

	configMapReconciler := configmap.NewConfigMapReconciler(client, builder)
	secretReconciler := secret.NewSecretReconciler(client, builder)
	serviceReconciler := service.NewServiceReconciler(client)
	endpointsReconciler := endpoints.NewEndpointsReconciler(client, builder)
	batchReconciler := batch.NewBatchReconciler(client, builder)
	rbacReconciler := rbac.NewRBACReconiler(client, builder)

	replConfig := replication.NewReplicationConfig(client, builder, secretReconciler)
	replicationReconciler := replication.NewReplicationReconciler(
		client,
		replRecorder,
		builder,
		replConfig,
		replication.WithRefResolver(refResolver),
		replication.WithSecretReconciler(secretReconciler),
		replication.WithServiceReconciler(serviceReconciler),
	)
	galeraReconciler := galera.NewGaleraReconciler(
		client,
		galeraRecorder,
		env,
		builder,
		galera.WithRefResolver(refResolver),
		galera.WithConfigMapReconciler(configMapReconciler),
		galera.WithServiceReconciler(serviceReconciler),
	)

	podReplicationController := NewPodController(
		client,
		refResolver,
		NewPodReplicationController(
			client,
			replRecorder,
			builder,
			refResolver,
			replConfig,
		),
		[]string{
			annotation.MariadbAnnotation,
			annotation.ReplicationAnnotation,
		},
	)
	podGaleraController := NewPodController(
		client,
		refResolver,
		NewPodGaleraController(client, galeraRecorder),
		[]string{
			annotation.MariadbAnnotation,
			annotation.GaleraAnnotation,
		},
	)

	err = (&MariaDBReconciler{
		Client: client,
		Scheme: scheme,

		Builder:        builder,
		RefResolver:    refResolver,
		ConditionReady: conditionReady,

		ServiceMonitorReconciler: true,

		ConfigMapReconciler: configMapReconciler,
		SecretReconciler:    secretReconciler,
		ServiceReconciler:   serviceReconciler,
		EndpointsReconciler: endpointsReconciler,
		RBACReconciler:      rbacReconciler,

		ReplicationReconciler: replicationReconciler,
		GaleraReconciler:      galeraReconciler,
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	err = (&BackupReconciler{
		Client:            client,
		Scheme:            scheme,
		Builder:           builder,
		RefResolver:       refResolver,
		ConditionComplete: conditionComplete,
		BatchReconciler:   batchReconciler,
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	err = (&RestoreReconciler{
		Client:            client,
		Scheme:            scheme,
		Builder:           builder,
		RefResolver:       refResolver,
		ConditionComplete: conditionComplete,
		BatchReconciler:   batchReconciler,
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	err = (&UserReconciler{
		Client:         client,
		Scheme:         scheme,
		RefResolver:    refResolver,
		ConditionReady: conditionReady,
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	err = (&GrantReconciler{
		Client:         client,
		Scheme:         scheme,
		RefResolver:    refResolver,
		ConditionReady: conditionReady,
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	err = (&DatabaseReconciler{
		Client:         client,
		Scheme:         scheme,
		RefResolver:    refResolver,
		ConditionReady: conditionReady,
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	err = (&ConnectionReconciler{
		Client:          client,
		Scheme:          scheme,
		Builder:         builder,
		RefResolver:     refResolver,
		ConditionReady:  conditionReady,
		RequeueInterval: 5 * time.Second,
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	err = (&SqlJobReconciler{
		Client:              client,
		Scheme:              scheme,
		Builder:             builder,
		RefResolver:         refResolver,
		ConfigMapReconciler: configMapReconciler,
		ConditionComplete:   conditionComplete,
		RequeueInterval:     5 * time.Second,
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	err = podReplicationController.SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	err = podGaleraController.SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	err = (&StatefulSetGaleraReconciler{
		Client:      client,
		RefResolver: refResolver,
		Recorder:    galeraRecorder,
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	go func() {
		defer GinkgoRecover()
		err = k8sManager.Start(testCtx)
		Expect(err).ToNot(HaveOccurred())
	}()

	By("Creating initial test data")
	createTestData(testCtx, k8sClient)
})

var _ = AfterSuite(func() {
	By("Deleting initial test data")
	deleteTestData(testCtx, k8sClient)

	testCancel()
	By("Tearing down the test environment")
	Expect(testEnv.Stop()).To(Succeed())
})
