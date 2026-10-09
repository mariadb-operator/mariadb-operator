package controller

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/builder"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/discovery"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/environment"
	stsobj "github.com/mariadb-operator/mariadb-operator/v26/pkg/statefulset"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestInitJobsAreNotCreatedAgainForRunningPods(t *testing.T) {
	for _, tc := range []struct {
		name     string
		running  int32
		wantJobs []int
		wantDone bool
	}{
		{name: "no Pod runs yet", running: 0, wantJobs: []int{0}},
		{name: "one Pod runs", running: 1, wantJobs: []int{1}},
		{name: "every Pod runs", running: 3, wantJobs: nil, wantDone: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mdb, objs := restoredMariaDB(tc.running)
			r, c := initJobsReconciler(t, append(objs, mdb)...)

			result, err := r.reconcileRollingInitJobs(context.Background(), mdb, 0, logr.Discard(), builder.WithBootstrapFrom(physicalBootstrap()))
			if err != nil {
				t.Fatalf("reconciling init jobs: %v", err)
			}
			if done := result.IsZero(); done != tc.wantDone {
				t.Errorf("init done = %v, want %v", done, tc.wantDone)
			}

			var jobs batchv1.JobList
			if err := c.List(context.Background(), &jobs); err != nil {
				t.Fatal(err)
			}
			want := make(map[string]bool)
			for _, i := range tc.wantJobs {
				want[mdb.PhysicalBackupInitJobKey(i).Name] = true
			}
			got := make(map[string]bool)
			for _, job := range jobs.Items {
				got[job.Name] = true
				if !want[job.Name] {
					t.Errorf("init job %s created, but its Pod is already running and holds its volume", job.Name)
				}
			}
			for name := range want {
				if !got[name] {
					t.Errorf("init job %s not created", name)
				}
			}
		})
	}
}

func restoredMariaDB(running int32) (*mariadbv1alpha1.MariaDB, []client.Object) {
	mdb := &mariadbv1alpha1.MariaDB{
		ObjectMeta: metav1.ObjectMeta{Name: "restored", Namespace: "test"},
		Spec: mariadbv1alpha1.MariaDBSpec{
			Replicas: 3,
			Image:    "mariadb:test",
			Storage:  mariadbv1alpha1.Storage{Size: ptr.To(resource.MustParse("1Gi"))},
		},
	}
	if running == 0 {
		return mdb, nil
	}
	objs := []client.Object{&appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: mdb.Name, Namespace: mdb.Namespace},
		Spec:       appsv1.StatefulSetSpec{Replicas: ptr.To(running)},
		Status:     appsv1.StatefulSetStatus{Replicas: running},
	}}
	for i := range int(running) {
		objs = append(objs, &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: stsobj.PodName(mdb.ObjectMeta, i), Namespace: mdb.Namespace},
			Spec:       corev1.PodSpec{NodeName: "node"},
		})
	}
	return mdb, objs
}

func physicalBootstrap() *mariadbv1alpha1.BootstrapFrom {
	return &mariadbv1alpha1.BootstrapFrom{
		BackupContentType: mariadbv1alpha1.BackupContentTypePhysical,
		S3:                &mariadbv1alpha1.S3{Bucket: "backups", Endpoint: "s3.example.net", Prefix: "source"},
		Volume:            &mariadbv1alpha1.StorageVolumeSource{EmptyDir: &mariadbv1alpha1.EmptyDirVolumeSource{}},
	}
}

func initJobsReconciler(t *testing.T, objs ...client.Object) (*MariaDBReconciler, client.Client) {
	t.Helper()
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(mariadbv1alpha1.AddToScheme(scheme))
	disc, err := discovery.NewFakeDiscovery()
	if err != nil {
		t.Fatal(err)
	}
	env := &environment.OperatorEnv{MariadbOperatorImage: "mariadb-operator:test", RelatedMariadbImage: "mariadb:test"}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).WithStatusSubresource(&appsv1.StatefulSet{}).Build()
	return &MariaDBReconciler{Client: c, Scheme: scheme, Builder: builder.NewBuilder(scheme, env, disc)}, c
}
