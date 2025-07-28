package builder

import (
	"errors"
	"fmt"
	"reflect"
	"testing"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v25/api/v1alpha1"
	labels "github.com/mariadb-operator/mariadb-operator/v25/pkg/builder/labels"
	galeraresources "github.com/mariadb-operator/mariadb-operator/v25/pkg/controller/galera/resources"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestBackupJobImagePullSecrets(t *testing.T) {
	builder := newDefaultTestBuilder(t)
	objMeta := metav1.ObjectMeta{
		Name:      "backup-image-pull-secrets",
		Namespace: "test",
	}

	tests := []struct {
		name            string
		backup          *mariadbv1alpha1.Backup
		mariadb         *mariadbv1alpha1.MariaDB
		wantPullSecrets []corev1.LocalObjectReference
	}{
		{
			name: "No Secrets",
			backup: &mariadbv1alpha1.Backup{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.BackupSpec{
					MariaDBRef: mariadbv1alpha1.MariaDBRef{
						ObjectReference: mariadbv1alpha1.ObjectReference{
							Name: objMeta.Name,
						},
					},
					Storage: mariadbv1alpha1.BackupStorage{
						S3: &mariadbv1alpha1.S3{},
					},
				},
			},
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec:       mariadbv1alpha1.MariaDBSpec{},
			},
			wantPullSecrets: nil,
		},
		{
			name: "Secrets in MariaDB",
			backup: &mariadbv1alpha1.Backup{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.BackupSpec{
					MariaDBRef: mariadbv1alpha1.MariaDBRef{
						ObjectReference: mariadbv1alpha1.ObjectReference{
							Name: objMeta.Name,
						},
					},
					Storage: mariadbv1alpha1.BackupStorage{
						S3: &mariadbv1alpha1.S3{},
					},
				},
			},
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					PodTemplate: mariadbv1alpha1.PodTemplate{
						ImagePullSecrets: []mariadbv1alpha1.LocalObjectReference{
							{
								Name: "mariadb-registry",
							},
						},
					},
				},
			},
			wantPullSecrets: []corev1.LocalObjectReference{
				{
					Name: "mariadb-registry",
				},
			},
		},
		{
			name: "Secrets in Backup",
			backup: &mariadbv1alpha1.Backup{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.BackupSpec{
					JobPodTemplate: mariadbv1alpha1.JobPodTemplate{
						ImagePullSecrets: []mariadbv1alpha1.LocalObjectReference{
							{
								Name: "backup-registry",
							},
						},
					},
					MariaDBRef: mariadbv1alpha1.MariaDBRef{
						ObjectReference: mariadbv1alpha1.ObjectReference{
							Name: objMeta.Name,
						},
					},
					Storage: mariadbv1alpha1.BackupStorage{
						S3: &mariadbv1alpha1.S3{},
					},
				},
			},
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec:       mariadbv1alpha1.MariaDBSpec{},
			},
			wantPullSecrets: []corev1.LocalObjectReference{
				{
					Name: "backup-registry",
				},
			},
		},
		{
			name: "Secrets in MariaDB and Backup",
			backup: &mariadbv1alpha1.Backup{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.BackupSpec{
					JobPodTemplate: mariadbv1alpha1.JobPodTemplate{
						ImagePullSecrets: []mariadbv1alpha1.LocalObjectReference{
							{
								Name: "backup-registry",
							},
						},
					},
					MariaDBRef: mariadbv1alpha1.MariaDBRef{
						ObjectReference: mariadbv1alpha1.ObjectReference{
							Name: objMeta.Name,
						},
					},
					Storage: mariadbv1alpha1.BackupStorage{
						S3: &mariadbv1alpha1.S3{},
					},
				},
			},
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					PodTemplate: mariadbv1alpha1.PodTemplate{
						ImagePullSecrets: []mariadbv1alpha1.LocalObjectReference{
							{
								Name: "mariadb-registry",
							},
						},
					},
				},
			},
			wantPullSecrets: []corev1.LocalObjectReference{
				{
					Name: "mariadb-registry",
				},
				{
					Name: "backup-registry",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job, err := builder.BuildBackupJob(client.ObjectKeyFromObject(tt.backup), tt.backup, tt.mariadb)
			if err != nil {
				t.Fatalf("unexpected error building Job: %v", err)
			}
			if !reflect.DeepEqual(tt.wantPullSecrets, job.Spec.Template.Spec.ImagePullSecrets) {
				t.Errorf("unexpected ImagePullSecrets, want: %v  got: %v", tt.wantPullSecrets, job.Spec.Template.Spec.ImagePullSecrets)
			}
		})
	}
}

// While this test tests mainly the kubernetes_volume_types.go implementation, it still makes a lot of sense to do it
// here as we get the bonus test coverage if the job building code correctly creates our volume sources. Because of this
// we only need to do this for backup and not restore.
// NOTE: We are using a lot of reflection to also capture cases in which a new field is added to the StorageVolumeSource
// but simply not properly implemented in any of the remaining code. If we would only test for static fields, this test
// would still pass while this new field would not be properly covered by a test.
func TestBackupJobVolumeSource(t *testing.T) {
	builder := newDefaultTestBuilder(t)
	objMeta := metav1.ObjectMeta{
		Name:      "backup-volume-source",
		Namespace: "test",
	}

	mariadb := &mariadbv1alpha1.MariaDB{
		ObjectMeta: objMeta,
		Spec:       mariadbv1alpha1.MariaDBSpec{},
	}

	// To make our testing easier (see our reflection code below), we define a single volume source that has ALL volume
	// source fields set!
	// NOTE: Our test does NOT check if the actual values are correct in the final job and corev1.VolumeSource.
	volumeSources := mariadbv1alpha1.StorageVolumeSource{
		EmptyDir: &mariadbv1alpha1.EmptyDirVolumeSource{},
		NFS: &mariadbv1alpha1.NFSVolumeSource{
			Server:   "test",
			Path:     "/some/thing",
			ReadOnly: true,
		},
		CSI: &mariadbv1alpha1.CSIVolumeSource{
			Driver: "test",
		},
		HostPath: &mariadbv1alpha1.HostPathVolumeSource{
			Path: "/some/path",
			Type: ptr.To(string(corev1.HostPathDirectoryOrCreate)),
		},
		PersistentVolumeClaim: &mariadbv1alpha1.PersistentVolumeClaimVolumeSource{
			ClaimName: "test-pvc",
		},
	}

	storageVolumeSourceType := reflect.TypeOf(volumeSources)
	storageVolumeSourceValue := reflect.ValueOf(volumeSources)

	for i := 0; i < storageVolumeSourceType.NumField(); i++ {
		field := storageVolumeSourceType.Field(i)

		// To prevent our code from being too fragile (as many of the copy code uses ifs without early aborts), we want
		// to create a plain StorageVolumeSource with only a single field set. So we need to "dynamically" copy over
		// from our volumeSources into this new volume source.
		volumeSource := mariadbv1alpha1.StorageVolumeSource{}
		reflect.ValueOf(&volumeSource).Elem().FieldByName(field.Name).Set(storageVolumeSourceValue.FieldByName(field.Name))

		t.Run(field.Name, func(t *testing.T) {
			backup := &mariadbv1alpha1.Backup{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.BackupSpec{
					MariaDBRef: mariadbv1alpha1.MariaDBRef{
						ObjectReference: mariadbv1alpha1.ObjectReference{
							Name: objMeta.Name,
						},
					},
					Storage: mariadbv1alpha1.BackupStorage{
						Volume: &volumeSource,
					},
				},
			}

			job, err := builder.BuildBackupJob(client.ObjectKeyFromObject(backup), backup, mariadb)
			if err != nil {
				t.Fatalf("unexpected error building Job: %v", err)
			}

			coreVolumeSourceValue := reflect.ValueOf(*getVolumeSource(batchStorageVolume, job))
			if coreVolumeSourceValue.FieldByName(field.Name).IsNil() {
				// NOTE: Ensure, the field is copied in `func (v StorageVolumeSource) ToKubernetesType()
				// corev1.VolumeSource`.
				t.Fatalf("The volume source field '%s' is not properly implemented as it is nil in corev1.VolumeSource.", field.Name)
			}

		})
	}

}

func TestBackupJobMeta(t *testing.T) {
	builder := newDefaultTestBuilder(t)
	key := types.NamespacedName{
		Name: "backup-job",
	}
	tests := []struct {
		name        string
		backup      *mariadbv1alpha1.Backup
		wantJobMeta *mariadbv1alpha1.Metadata
		wantPodMeta *mariadbv1alpha1.Metadata
	}{
		{
			name: "empty",
			backup: &mariadbv1alpha1.Backup{
				Spec: mariadbv1alpha1.BackupSpec{
					Storage: mariadbv1alpha1.BackupStorage{
						PersistentVolumeClaim: &mariadbv1alpha1.PersistentVolumeClaimSpec{},
					},
				},
			},
			wantJobMeta: &mariadbv1alpha1.Metadata{
				Labels:      map[string]string{},
				Annotations: map[string]string{},
			},
			wantPodMeta: &mariadbv1alpha1.Metadata{
				Labels:      map[string]string{},
				Annotations: map[string]string{},
			},
		},
		{
			name: "inherit metadata",
			backup: &mariadbv1alpha1.Backup{
				Spec: mariadbv1alpha1.BackupSpec{
					Storage: mariadbv1alpha1.BackupStorage{
						PersistentVolumeClaim: &mariadbv1alpha1.PersistentVolumeClaimSpec{},
					},
					InheritMetadata: &mariadbv1alpha1.Metadata{
						Labels: map[string]string{
							"sidecar.istio.io/inject": "false",
						},
						Annotations: map[string]string{
							"database.myorg.io": "mariadb",
						},
					},
				},
			},
			wantJobMeta: &mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"sidecar.istio.io/inject": "false",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
			wantPodMeta: &mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"sidecar.istio.io/inject": "false",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
		},
		{
			name: "Pod meta",
			backup: &mariadbv1alpha1.Backup{
				Spec: mariadbv1alpha1.BackupSpec{
					Storage: mariadbv1alpha1.BackupStorage{
						PersistentVolumeClaim: &mariadbv1alpha1.PersistentVolumeClaimSpec{},
					},
					JobPodTemplate: mariadbv1alpha1.JobPodTemplate{
						PodMetadata: &mariadbv1alpha1.Metadata{
							Labels: map[string]string{
								"sidecar.istio.io/inject": "false",
							},
							Annotations: map[string]string{
								"database.myorg.io": "mariadb",
							},
						},
					},
				},
			},
			wantJobMeta: &mariadbv1alpha1.Metadata{
				Labels:      map[string]string{},
				Annotations: map[string]string{},
			},
			wantPodMeta: &mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"sidecar.istio.io/inject": "false",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
		},
		{
			name: "override inherit metadata",
			backup: &mariadbv1alpha1.Backup{
				Spec: mariadbv1alpha1.BackupSpec{
					Storage: mariadbv1alpha1.BackupStorage{
						PersistentVolumeClaim: &mariadbv1alpha1.PersistentVolumeClaimSpec{},
					},
					InheritMetadata: &mariadbv1alpha1.Metadata{
						Labels: map[string]string{
							"sidecar.istio.io/inject": "true",
						},
						Annotations: map[string]string{
							"database.myorg.io": "mariadb",
						},
					},
					JobPodTemplate: mariadbv1alpha1.JobPodTemplate{
						PodMetadata: &mariadbv1alpha1.Metadata{
							Labels: map[string]string{
								"sidecar.istio.io/inject": "false",
							},
							Annotations: map[string]string{
								"database.myorg.io": "mariadb",
							},
						},
					},
				},
			},
			wantJobMeta: &mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"sidecar.istio.io/inject": "true",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
			wantPodMeta: &mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"sidecar.istio.io/inject": "false",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
		},
		{
			name: "all",
			backup: &mariadbv1alpha1.Backup{
				Spec: mariadbv1alpha1.BackupSpec{
					Storage: mariadbv1alpha1.BackupStorage{
						PersistentVolumeClaim: &mariadbv1alpha1.PersistentVolumeClaimSpec{},
					},
					InheritMetadata: &mariadbv1alpha1.Metadata{
						Annotations: map[string]string{
							"database.myorg.io": "mariadb",
						},
					},
					JobPodTemplate: mariadbv1alpha1.JobPodTemplate{
						PodMetadata: &mariadbv1alpha1.Metadata{
							Labels: map[string]string{
								"sidecar.istio.io/inject": "false",
							},
						},
					},
				},
			},
			wantJobMeta: &mariadbv1alpha1.Metadata{
				Labels: map[string]string{},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
			wantPodMeta: &mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"sidecar.istio.io/inject": "false",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job, err := builder.BuildBackupJob(key, tt.backup, &mariadbv1alpha1.MariaDB{})
			if err != nil {
				t.Fatalf("unexpected error building Backup Job: %v", err)
			}
			assertObjectMeta(t, &job.ObjectMeta, tt.wantJobMeta.Labels, tt.wantJobMeta.Annotations)
			assertObjectMeta(t, &job.Spec.Template.ObjectMeta, tt.wantPodMeta.Labels, tt.wantPodMeta.Annotations)
		})
	}
}

func TestPhysicalBackupJobNodeSelector(t *testing.T) {
	builder := newDefaultTestBuilder(t)
	podObjMeta := metav1.ObjectMeta{
		Name: "mariadb-0",
	}
	mariadb := &mariadbv1alpha1.MariaDB{
		ObjectMeta: metav1.ObjectMeta{
			Name: "physicalbackup-test",
		},
		Spec: mariadbv1alpha1.MariaDBSpec{},
	}

	tests := []struct {
		name             string
		backup           *mariadbv1alpha1.PhysicalBackup
		pod              *corev1.Pod
		wantErr          bool
		wantNodeSelector bool
	}{
		{
			name:   "error when pod nodeName is empty",
			backup: &mariadbv1alpha1.PhysicalBackup{},
			pod: &corev1.Pod{
				ObjectMeta: podObjMeta,
				Spec: corev1.PodSpec{
					NodeName: "",
				},
			},
			wantErr:          true,
			wantNodeSelector: false,
		},
		{
			name: "nodeSelector set when podAffinity is true (default)",
			backup: &mariadbv1alpha1.PhysicalBackup{
				Spec: mariadbv1alpha1.PhysicalBackupSpec{
					Storage: mariadbv1alpha1.PhysicalBackupStorage{
						Volume: &mariadbv1alpha1.StorageVolumeSource{
							EmptyDir: &mariadbv1alpha1.EmptyDirVolumeSource{},
						},
					},
				},
			},
			pod: &corev1.Pod{
				ObjectMeta: podObjMeta,
				Spec: corev1.PodSpec{
					NodeName: "node-1",
				},
			},
			wantErr:          false,
			wantNodeSelector: true,
		},
		{
			name: "nodeSelector set when podAffinity is true (explicit)",
			backup: &mariadbv1alpha1.PhysicalBackup{
				Spec: mariadbv1alpha1.PhysicalBackupSpec{
					PodAffinity: ptr.To(true),
					Storage: mariadbv1alpha1.PhysicalBackupStorage{
						Volume: &mariadbv1alpha1.StorageVolumeSource{
							EmptyDir: &mariadbv1alpha1.EmptyDirVolumeSource{},
						},
					},
				},
			},
			pod: &corev1.Pod{
				ObjectMeta: podObjMeta,
				Spec: corev1.PodSpec{
					NodeName: "node-1",
				},
			},
			wantErr:          false,
			wantNodeSelector: true,
		},
		{
			name: "nodeSelector not set when podAffinity is false",
			backup: &mariadbv1alpha1.PhysicalBackup{
				Spec: mariadbv1alpha1.PhysicalBackupSpec{
					PodAffinity: ptr.To(false),
					Storage: mariadbv1alpha1.PhysicalBackupStorage{
						Volume: &mariadbv1alpha1.StorageVolumeSource{
							EmptyDir: &mariadbv1alpha1.EmptyDirVolumeSource{},
						},
					},
				},
			},
			pod: &corev1.Pod{
				ObjectMeta: podObjMeta,
				Spec: corev1.PodSpec{
					NodeName: "node-1",
				},
			},
			wantErr:          false,
			wantNodeSelector: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job, err := builder.BuildPhysicalBackupJob(
				client.ObjectKeyFromObject(tt.backup),
				tt.backup,
				mariadb,
				tt.pod,
				"backupfile",
			)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error but got none")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			nodeSelector := job.Spec.Template.Spec.NodeSelector
			if tt.wantNodeSelector {
				assert.NotNil(t, nodeSelector, "expected nodeSelector to be set")
				assert.Equal(
					t,
					tt.pod.Spec.NodeName,
					nodeSelector["kubernetes.io/hostname"],
					errors.New("expected nodeSelector to be set to pod's nodeName"),
				)
			} else {
				assert.True(
					t,
					nodeSelector == nil || nodeSelector["kubernetes.io/hostname"] == "",
					fmt.Errorf("expected nodeSelector to be nil or not set, got %v", nodeSelector),
				)
			}
		})
	}
}

func TestPhysicalBackupJobMeta(t *testing.T) {
	builder := newDefaultTestBuilder(t)
	key := types.NamespacedName{
		Name: "physical-backup-job",
	}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "mariadb-0",
		},
		Spec: corev1.PodSpec{
			NodeName: "node-1",
		},
	}
	tests := []struct {
		name        string
		backup      *mariadbv1alpha1.PhysicalBackup
		wantJobMeta *mariadbv1alpha1.Metadata
		wantPodMeta *mariadbv1alpha1.Metadata
	}{
		{
			name: "empty",
			backup: &mariadbv1alpha1.PhysicalBackup{
				Spec: mariadbv1alpha1.PhysicalBackupSpec{
					Storage: mariadbv1alpha1.PhysicalBackupStorage{
						Volume: &mariadbv1alpha1.StorageVolumeSource{
							PersistentVolumeClaim: &mariadbv1alpha1.PersistentVolumeClaimVolumeSource{},
						},
					},
				},
			},
			wantJobMeta: &mariadbv1alpha1.Metadata{
				Labels:      map[string]string{},
				Annotations: map[string]string{},
			},
			wantPodMeta: &mariadbv1alpha1.Metadata{
				Labels:      map[string]string{},
				Annotations: map[string]string{},
			},
		},
		{
			name: "inherit metadata",
			backup: &mariadbv1alpha1.PhysicalBackup{
				Spec: mariadbv1alpha1.PhysicalBackupSpec{
					Storage: mariadbv1alpha1.PhysicalBackupStorage{
						Volume: &mariadbv1alpha1.StorageVolumeSource{
							PersistentVolumeClaim: &mariadbv1alpha1.PersistentVolumeClaimVolumeSource{},
						},
					},
					InheritMetadata: &mariadbv1alpha1.Metadata{
						Labels: map[string]string{
							"sidecar.istio.io/inject": "false",
						},
						Annotations: map[string]string{
							"database.myorg.io": "mariadb",
						},
					},
				},
			},
			wantJobMeta: &mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"sidecar.istio.io/inject": "false",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
			wantPodMeta: &mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"sidecar.istio.io/inject": "false",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
		},
		{
			name: "Pod meta",
			backup: &mariadbv1alpha1.PhysicalBackup{
				Spec: mariadbv1alpha1.PhysicalBackupSpec{
					Storage: mariadbv1alpha1.PhysicalBackupStorage{
						Volume: &mariadbv1alpha1.StorageVolumeSource{
							PersistentVolumeClaim: &mariadbv1alpha1.PersistentVolumeClaimVolumeSource{},
						},
					},
					PhysicalBackupPodTemplate: mariadbv1alpha1.PhysicalBackupPodTemplate{
						PodMetadata: &mariadbv1alpha1.Metadata{
							Labels: map[string]string{
								"sidecar.istio.io/inject": "false",
							},
							Annotations: map[string]string{
								"database.myorg.io": "mariadb",
							},
						},
					},
				},
			},
			wantJobMeta: &mariadbv1alpha1.Metadata{
				Labels:      map[string]string{},
				Annotations: map[string]string{},
			},
			wantPodMeta: &mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"sidecar.istio.io/inject": "false",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
		},
		{
			name: "override inherit metadata",
			backup: &mariadbv1alpha1.PhysicalBackup{
				Spec: mariadbv1alpha1.PhysicalBackupSpec{
					Storage: mariadbv1alpha1.PhysicalBackupStorage{
						Volume: &mariadbv1alpha1.StorageVolumeSource{
							PersistentVolumeClaim: &mariadbv1alpha1.PersistentVolumeClaimVolumeSource{},
						},
					},
					InheritMetadata: &mariadbv1alpha1.Metadata{
						Labels: map[string]string{
							"sidecar.istio.io/inject": "true",
						},
						Annotations: map[string]string{
							"database.myorg.io": "mariadb",
						},
					},
					PhysicalBackupPodTemplate: mariadbv1alpha1.PhysicalBackupPodTemplate{
						PodMetadata: &mariadbv1alpha1.Metadata{
							Labels: map[string]string{
								"sidecar.istio.io/inject": "false",
							},
							Annotations: map[string]string{
								"database.myorg.io": "mariadb",
							},
						},
					},
				},
			},
			wantJobMeta: &mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"sidecar.istio.io/inject": "true",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
			wantPodMeta: &mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"sidecar.istio.io/inject": "false",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
		},
		{
			name: "all",
			backup: &mariadbv1alpha1.PhysicalBackup{
				Spec: mariadbv1alpha1.PhysicalBackupSpec{
					Storage: mariadbv1alpha1.PhysicalBackupStorage{
						Volume: &mariadbv1alpha1.StorageVolumeSource{
							PersistentVolumeClaim: &mariadbv1alpha1.PersistentVolumeClaimVolumeSource{},
						},
					},
					InheritMetadata: &mariadbv1alpha1.Metadata{
						Annotations: map[string]string{
							"database.myorg.io": "mariadb",
						},
					},
					PhysicalBackupPodTemplate: mariadbv1alpha1.PhysicalBackupPodTemplate{
						PodMetadata: &mariadbv1alpha1.Metadata{
							Labels: map[string]string{
								"sidecar.istio.io/inject": "false",
							},
						},
					},
				},
			},
			wantJobMeta: &mariadbv1alpha1.Metadata{
				Labels: map[string]string{},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
			wantPodMeta: &mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"sidecar.istio.io/inject": "false",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job, err := builder.BuildPhysicalBackupJob(key, tt.backup, &mariadbv1alpha1.MariaDB{}, pod, "backupfile")
			if err != nil {
				t.Fatalf("unexpected error building PhysicalBackup Job: %v", err)
			}
			assertObjectMeta(t, &job.ObjectMeta, tt.wantJobMeta.Labels, tt.wantJobMeta.Annotations)
			assertObjectMeta(t, &job.Spec.Template.ObjectMeta, tt.wantPodMeta.Labels, tt.wantPodMeta.Annotations)
		})
	}
}

func TestPhysicalBackupJobImagePullSecrets(t *testing.T) {
	builder := newDefaultTestBuilder(t)
	objMeta := metav1.ObjectMeta{
		Name:      "physical-backup-image-pull-secrets",
		Namespace: "test",
	}
	mariadb := &mariadbv1alpha1.MariaDB{
		ObjectMeta: objMeta,
		Spec:       mariadbv1alpha1.MariaDBSpec{},
	}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "mariadb-0",
		},
		Spec: corev1.PodSpec{
			NodeName: "node-1",
		},
	}

	tests := []struct {
		name            string
		backup          *mariadbv1alpha1.PhysicalBackup
		mariadb         *mariadbv1alpha1.MariaDB
		wantPullSecrets []corev1.LocalObjectReference
	}{
		{
			name: "No Secrets",
			backup: &mariadbv1alpha1.PhysicalBackup{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.PhysicalBackupSpec{
					Storage: mariadbv1alpha1.PhysicalBackupStorage{
						Volume: &mariadbv1alpha1.StorageVolumeSource{
							EmptyDir: &mariadbv1alpha1.EmptyDirVolumeSource{},
						},
					},
				},
			},
			mariadb:         mariadb,
			wantPullSecrets: nil,
		},
		{
			name: "Secrets in MariaDB",
			backup: &mariadbv1alpha1.PhysicalBackup{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.PhysicalBackupSpec{
					Storage: mariadbv1alpha1.PhysicalBackupStorage{
						Volume: &mariadbv1alpha1.StorageVolumeSource{
							EmptyDir: &mariadbv1alpha1.EmptyDirVolumeSource{},
						},
					},
				},
			},
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					PodTemplate: mariadbv1alpha1.PodTemplate{
						ImagePullSecrets: []mariadbv1alpha1.LocalObjectReference{
							{
								Name: "mariadb-registry",
							},
						},
					},
				},
			},
			wantPullSecrets: []corev1.LocalObjectReference{
				{
					Name: "mariadb-registry",
				},
			},
		},
		{
			name: "Secrets in PhysicalBackup",
			backup: &mariadbv1alpha1.PhysicalBackup{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.PhysicalBackupSpec{
					PhysicalBackupPodTemplate: mariadbv1alpha1.PhysicalBackupPodTemplate{
						ImagePullSecrets: []mariadbv1alpha1.LocalObjectReference{
							{
								Name: "physicalbackup-registry",
							},
						},
					},
					Storage: mariadbv1alpha1.PhysicalBackupStorage{
						Volume: &mariadbv1alpha1.StorageVolumeSource{
							EmptyDir: &mariadbv1alpha1.EmptyDirVolumeSource{},
						},
					},
				},
			},
			mariadb: mariadb,
			wantPullSecrets: []corev1.LocalObjectReference{
				{
					Name: "physicalbackup-registry",
				},
			},
		},
		{
			name: "Secrets in MariaDB and PhysicalBackup",
			backup: &mariadbv1alpha1.PhysicalBackup{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.PhysicalBackupSpec{
					PhysicalBackupPodTemplate: mariadbv1alpha1.PhysicalBackupPodTemplate{
						ImagePullSecrets: []mariadbv1alpha1.LocalObjectReference{
							{
								Name: "physicalbackup-registry",
							},
						},
					},
					Storage: mariadbv1alpha1.PhysicalBackupStorage{
						Volume: &mariadbv1alpha1.StorageVolumeSource{
							EmptyDir: &mariadbv1alpha1.EmptyDirVolumeSource{},
						},
					},
				},
			},
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					PodTemplate: mariadbv1alpha1.PodTemplate{
						ImagePullSecrets: []mariadbv1alpha1.LocalObjectReference{
							{
								Name: "mariadb-registry",
							},
						},
					},
				},
			},
			wantPullSecrets: []corev1.LocalObjectReference{
				{
					Name: "mariadb-registry",
				},
				{
					Name: "physicalbackup-registry",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job, err := builder.BuildPhysicalBackupJob(
				client.ObjectKeyFromObject(tt.backup),
				tt.backup,
				tt.mariadb,
				pod,
				"backupfile",
			)
			if err != nil {
				t.Fatalf("unexpected error building Job: %v", err)
			}
			if !reflect.DeepEqual(tt.wantPullSecrets, job.Spec.Template.Spec.ImagePullSecrets) {
				t.Errorf("unexpected ImagePullSecrets, want: %v  got: %v", tt.wantPullSecrets, job.Spec.Template.Spec.ImagePullSecrets)
			}
		})
	}
}

func TestRestoreJobImagePullSecrets(t *testing.T) {
	builder := newDefaultTestBuilder(t)
	objMeta := metav1.ObjectMeta{
		Name:      "restore-image-pull-secrets",
		Namespace: "test",
	}

	tests := []struct {
		name            string
		restore         *mariadbv1alpha1.Restore
		mariadb         *mariadbv1alpha1.MariaDB
		wantPullSecrets []corev1.LocalObjectReference
	}{
		{
			name: "No Secrets",
			restore: &mariadbv1alpha1.Restore{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.RestoreSpec{
					MariaDBRef: mariadbv1alpha1.MariaDBRef{
						ObjectReference: mariadbv1alpha1.ObjectReference{
							Name: objMeta.Name,
						},
					},
					RestoreSource: mariadbv1alpha1.RestoreSource{
						Volume: &mariadbv1alpha1.StorageVolumeSource{},
					},
				},
			},
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec:       mariadbv1alpha1.MariaDBSpec{},
			},
			wantPullSecrets: nil,
		},
		{
			name: "Secrets in MariaDB",
			restore: &mariadbv1alpha1.Restore{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.RestoreSpec{
					MariaDBRef: mariadbv1alpha1.MariaDBRef{
						ObjectReference: mariadbv1alpha1.ObjectReference{
							Name: objMeta.Name,
						},
					},
					RestoreSource: mariadbv1alpha1.RestoreSource{
						Volume: &mariadbv1alpha1.StorageVolumeSource{},
					},
				},
			},
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					PodTemplate: mariadbv1alpha1.PodTemplate{
						ImagePullSecrets: []mariadbv1alpha1.LocalObjectReference{
							{
								Name: "mariadb-registry",
							},
						},
					},
				},
			},
			wantPullSecrets: []corev1.LocalObjectReference{
				{
					Name: "mariadb-registry",
				},
			},
		},
		{
			name: "Secrets in Restore",
			restore: &mariadbv1alpha1.Restore{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.RestoreSpec{
					JobPodTemplate: mariadbv1alpha1.JobPodTemplate{
						ImagePullSecrets: []mariadbv1alpha1.LocalObjectReference{
							{
								Name: "restore-registry",
							},
						},
					},
					RestoreSource: mariadbv1alpha1.RestoreSource{
						Volume: &mariadbv1alpha1.StorageVolumeSource{},
					},
					MariaDBRef: mariadbv1alpha1.MariaDBRef{
						ObjectReference: mariadbv1alpha1.ObjectReference{
							Name: objMeta.Name,
						},
					},
				},
			},
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec:       mariadbv1alpha1.MariaDBSpec{},
			},
			wantPullSecrets: []corev1.LocalObjectReference{
				{
					Name: "restore-registry",
				},
			},
		},
		{
			name: "Secrets in MariaDB and Restore",
			restore: &mariadbv1alpha1.Restore{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.RestoreSpec{
					JobPodTemplate: mariadbv1alpha1.JobPodTemplate{
						ImagePullSecrets: []mariadbv1alpha1.LocalObjectReference{
							{
								Name: "restore-registry",
							},
						},
					},
					RestoreSource: mariadbv1alpha1.RestoreSource{
						Volume: &mariadbv1alpha1.StorageVolumeSource{},
					},
					MariaDBRef: mariadbv1alpha1.MariaDBRef{
						ObjectReference: mariadbv1alpha1.ObjectReference{
							Name: objMeta.Name,
						},
					},
				},
			},
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					PodTemplate: mariadbv1alpha1.PodTemplate{
						ImagePullSecrets: []mariadbv1alpha1.LocalObjectReference{
							{
								Name: "mariadb-registry",
							},
						},
					},
				},
			},
			wantPullSecrets: []corev1.LocalObjectReference{
				{
					Name: "mariadb-registry",
				},
				{
					Name: "restore-registry",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job, err := builder.BuildRestoreJob(client.ObjectKeyFromObject(tt.restore), tt.restore, tt.mariadb)
			if err != nil {
				t.Fatalf("unexpected error building Job: %v", err)
			}
			if !reflect.DeepEqual(tt.wantPullSecrets, job.Spec.Template.Spec.ImagePullSecrets) {
				t.Errorf("unexpected ImagePullSecrets, want: %v  got: %v", tt.wantPullSecrets, job.Spec.Template.Spec.ImagePullSecrets)
			}
		})
	}
}

func TestRestoreJobMeta(t *testing.T) {
	builder := newDefaultTestBuilder(t)
	key := types.NamespacedName{
		Name: "restore-job",
	}
	tests := []struct {
		name        string
		restore     *mariadbv1alpha1.Restore
		wantJobMeta *mariadbv1alpha1.Metadata
		wantPodMeta *mariadbv1alpha1.Metadata
	}{
		{
			name: "empty",
			restore: &mariadbv1alpha1.Restore{
				Spec: mariadbv1alpha1.RestoreSpec{
					RestoreSource: mariadbv1alpha1.RestoreSource{
						Volume: &mariadbv1alpha1.StorageVolumeSource{
							PersistentVolumeClaim: &mariadbv1alpha1.PersistentVolumeClaimVolumeSource{},
						},
						S3: &mariadbv1alpha1.S3{},
					},
				},
			},
			wantJobMeta: &mariadbv1alpha1.Metadata{
				Labels:      map[string]string{},
				Annotations: map[string]string{},
			},
			wantPodMeta: &mariadbv1alpha1.Metadata{
				Labels:      map[string]string{},
				Annotations: map[string]string{},
			},
		},
		{
			name: "inherit metadata",
			restore: &mariadbv1alpha1.Restore{
				Spec: mariadbv1alpha1.RestoreSpec{
					RestoreSource: mariadbv1alpha1.RestoreSource{
						Volume: &mariadbv1alpha1.StorageVolumeSource{
							PersistentVolumeClaim: &mariadbv1alpha1.PersistentVolumeClaimVolumeSource{},
						},
						S3: &mariadbv1alpha1.S3{},
					},
					InheritMetadata: &mariadbv1alpha1.Metadata{
						Labels: map[string]string{
							"sidecar.istio.io/inject": "false",
						},
						Annotations: map[string]string{
							"database.myorg.io": "mariadb",
						},
					},
				},
			},
			wantJobMeta: &mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"sidecar.istio.io/inject": "false",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
			wantPodMeta: &mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"sidecar.istio.io/inject": "false",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
		},
		{
			name: "Pod meta",
			restore: &mariadbv1alpha1.Restore{
				Spec: mariadbv1alpha1.RestoreSpec{
					RestoreSource: mariadbv1alpha1.RestoreSource{
						Volume: &mariadbv1alpha1.StorageVolumeSource{
							PersistentVolumeClaim: &mariadbv1alpha1.PersistentVolumeClaimVolumeSource{},
						},
						S3: &mariadbv1alpha1.S3{},
					},
					JobPodTemplate: mariadbv1alpha1.JobPodTemplate{
						PodMetadata: &mariadbv1alpha1.Metadata{
							Labels: map[string]string{
								"sidecar.istio.io/inject": "false",
							},
							Annotations: map[string]string{
								"database.myorg.io": "mariadb",
							},
						},
					},
				},
			},
			wantJobMeta: &mariadbv1alpha1.Metadata{
				Labels:      map[string]string{},
				Annotations: map[string]string{},
			},
			wantPodMeta: &mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"sidecar.istio.io/inject": "false",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
		},
		{
			name: "override inherit metadata",
			restore: &mariadbv1alpha1.Restore{
				Spec: mariadbv1alpha1.RestoreSpec{
					RestoreSource: mariadbv1alpha1.RestoreSource{
						Volume: &mariadbv1alpha1.StorageVolumeSource{
							PersistentVolumeClaim: &mariadbv1alpha1.PersistentVolumeClaimVolumeSource{},
						},
						S3: &mariadbv1alpha1.S3{},
					},
					InheritMetadata: &mariadbv1alpha1.Metadata{
						Labels: map[string]string{
							"sidecar.istio.io/inject": "true",
						},
						Annotations: map[string]string{
							"database.myorg.io": "mariadb",
						},
					},
					JobPodTemplate: mariadbv1alpha1.JobPodTemplate{
						PodMetadata: &mariadbv1alpha1.Metadata{
							Labels: map[string]string{
								"sidecar.istio.io/inject": "false",
							},
							Annotations: map[string]string{
								"database.myorg.io": "mariadb",
							},
						},
					},
				},
			},
			wantJobMeta: &mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"sidecar.istio.io/inject": "true",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
			wantPodMeta: &mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"sidecar.istio.io/inject": "false",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
		},
		{
			name: "all",
			restore: &mariadbv1alpha1.Restore{
				Spec: mariadbv1alpha1.RestoreSpec{
					RestoreSource: mariadbv1alpha1.RestoreSource{
						Volume: &mariadbv1alpha1.StorageVolumeSource{
							PersistentVolumeClaim: &mariadbv1alpha1.PersistentVolumeClaimVolumeSource{},
						},
						S3: &mariadbv1alpha1.S3{},
					},
					InheritMetadata: &mariadbv1alpha1.Metadata{
						Annotations: map[string]string{
							"database.myorg.io": "mariadb",
						},
					},
					JobPodTemplate: mariadbv1alpha1.JobPodTemplate{
						PodMetadata: &mariadbv1alpha1.Metadata{
							Labels: map[string]string{
								"sidecar.istio.io/inject": "false",
							},
						},
					},
				},
			},
			wantJobMeta: &mariadbv1alpha1.Metadata{
				Labels: map[string]string{},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
			wantPodMeta: &mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"sidecar.istio.io/inject": "false",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job, err := builder.BuildRestoreJob(key, tt.restore, &mariadbv1alpha1.MariaDB{})
			if err != nil {
				t.Fatalf("unexpected error building Restore Job: %v", err)
			}
			assertObjectMeta(t, &job.ObjectMeta, tt.wantJobMeta.Labels, tt.wantJobMeta.Annotations)
			assertObjectMeta(t, &job.Spec.Template.ObjectMeta, tt.wantPodMeta.Labels, tt.wantPodMeta.Annotations)
		})
	}
}

func TestPhysicalBackupRestoreJobSelectorLabels(t *testing.T) {
	builder := newDefaultTestBuilder(t)
	key := types.NamespacedName{
		Name:      "physical-backup-restore-job",
		Namespace: "test",
	}

	// Setup MariaDB with BootstrapFrom and selector labels
	mariadb := &mariadbv1alpha1.MariaDB{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mariadb-test",
			Namespace: "test",
		},
		Spec: mariadbv1alpha1.MariaDBSpec{
			BootstrapFrom: &mariadbv1alpha1.BootstrapFrom{
				Volume: &mariadbv1alpha1.StorageVolumeSource{
					EmptyDir: &mariadbv1alpha1.EmptyDirVolumeSource{},
				},
			},
		},
	}
	podIndex := ptr.To(0)

	job, err := builder.BuildPhysicalBackupRestoreJob(key, mariadb, podIndex)
	if err != nil {
		t.Fatalf("unexpected error building PhysicalBackupRestoreJob: %v", err)
	}

	// The selector labels should be present in the pod template metadata
	selectorLabels := labels.NewLabelsBuilder().WithMariaDBSelectorLabels(mariadb).Build()
	for k, v := range selectorLabels {
		got := job.Spec.Template.Labels[k]
		if got != v {
			t.Errorf("expected selector label %q=%q, got %q", k, v, got)
		}
	}
}

func TestPhysicalBackupRestoreJobMeta(t *testing.T) {
	builder := newDefaultTestBuilder(t)
	objMeta := metav1.ObjectMeta{
		Name: "mariadb-obj",
	}
	key := types.NamespacedName{
		Name:      "physical-backup-restore-job-meta",
		Namespace: "test",
	}

	tests := []struct {
		name        string
		mariadb     *mariadbv1alpha1.MariaDB
		wantJobMeta *mariadbv1alpha1.Metadata
		wantPodMeta *mariadbv1alpha1.Metadata
	}{
		{
			name: "empty",
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					BootstrapFrom: &mariadbv1alpha1.BootstrapFrom{
						Volume: &mariadbv1alpha1.StorageVolumeSource{
							PersistentVolumeClaim: &mariadbv1alpha1.PersistentVolumeClaimVolumeSource{},
						},
					},
				},
			},
			wantJobMeta: &mariadbv1alpha1.Metadata{
				Labels:      map[string]string{},
				Annotations: map[string]string{},
			},
			wantPodMeta: &mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"app.kubernetes.io/name":     "mariadb",
					"app.kubernetes.io/instance": "mariadb-obj",
				},
				Annotations: map[string]string{},
			},
		},
		{
			name: "inherit metadata",
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					BootstrapFrom: &mariadbv1alpha1.BootstrapFrom{
						Volume: &mariadbv1alpha1.StorageVolumeSource{
							PersistentVolumeClaim: &mariadbv1alpha1.PersistentVolumeClaimVolumeSource{},
						},
					},
					InheritMetadata: &mariadbv1alpha1.Metadata{
						Labels: map[string]string{
							"sidecar.istio.io/inject": "false",
						},
						Annotations: map[string]string{
							"database.myorg.io": "mariadb",
						},
					},
				},
			},
			wantJobMeta: &mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"sidecar.istio.io/inject": "false",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
			wantPodMeta: &mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"sidecar.istio.io/inject":    "false",
					"app.kubernetes.io/name":     "mariadb",
					"app.kubernetes.io/instance": "mariadb-obj",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
		},
		{
			name: "Pod meta",
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					BootstrapFrom: &mariadbv1alpha1.BootstrapFrom{
						Volume: &mariadbv1alpha1.StorageVolumeSource{
							PersistentVolumeClaim: &mariadbv1alpha1.PersistentVolumeClaimVolumeSource{},
						},
					},
					PodTemplate: mariadbv1alpha1.PodTemplate{
						PodMetadata: &mariadbv1alpha1.Metadata{
							Labels: map[string]string{
								"sidecar.istio.io/inject": "false",
							},
							Annotations: map[string]string{
								"database.myorg.io": "mariadb",
							},
						},
					},
				},
			},
			wantJobMeta: &mariadbv1alpha1.Metadata{
				Labels:      map[string]string{},
				Annotations: map[string]string{},
			},
			wantPodMeta: &mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"sidecar.istio.io/inject":    "false",
					"app.kubernetes.io/name":     "mariadb",
					"app.kubernetes.io/instance": "mariadb-obj",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
		},
		{
			name: "override inherit metadata",
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					BootstrapFrom: &mariadbv1alpha1.BootstrapFrom{
						Volume: &mariadbv1alpha1.StorageVolumeSource{
							PersistentVolumeClaim: &mariadbv1alpha1.PersistentVolumeClaimVolumeSource{},
						},
					},
					InheritMetadata: &mariadbv1alpha1.Metadata{
						Labels: map[string]string{
							"sidecar.istio.io/inject": "true",
						},
						Annotations: map[string]string{
							"database.myorg.io": "mariadb",
						},
					},
					PodTemplate: mariadbv1alpha1.PodTemplate{
						PodMetadata: &mariadbv1alpha1.Metadata{
							Labels: map[string]string{
								"sidecar.istio.io/inject":    "false",
								"app.kubernetes.io/name":     "mariadb",
								"app.kubernetes.io/instance": "mariadb-obj",
							},
							Annotations: map[string]string{
								"database.myorg.io": "mariadb",
							},
						},
					},
				},
			},
			wantJobMeta: &mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"sidecar.istio.io/inject": "true",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
			wantPodMeta: &mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"sidecar.istio.io/inject":    "false",
					"app.kubernetes.io/name":     "mariadb",
					"app.kubernetes.io/instance": "mariadb-obj",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
		},
		{
			name: "all",
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					BootstrapFrom: &mariadbv1alpha1.BootstrapFrom{
						Volume: &mariadbv1alpha1.StorageVolumeSource{
							PersistentVolumeClaim: &mariadbv1alpha1.PersistentVolumeClaimVolumeSource{},
						},
					},
					InheritMetadata: &mariadbv1alpha1.Metadata{
						Annotations: map[string]string{
							"database.myorg.io": "mariadb",
						},
					},
					PodTemplate: mariadbv1alpha1.PodTemplate{
						PodMetadata: &mariadbv1alpha1.Metadata{
							Labels: map[string]string{
								"sidecar.istio.io/inject": "false",
							},
						},
					},
				},
			},
			wantJobMeta: &mariadbv1alpha1.Metadata{
				Labels: map[string]string{},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
			wantPodMeta: &mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"sidecar.istio.io/inject":    "false",
					"app.kubernetes.io/name":     "mariadb",
					"app.kubernetes.io/instance": "mariadb-obj",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
		},
	}

	podIndex := ptr.To(0)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job, err := builder.BuildPhysicalBackupRestoreJob(key, tt.mariadb, podIndex)
			if err != nil {
				t.Fatalf("unexpected error building PhysicalBackupRestoreJob: %v", err)
			}
			assertObjectMeta(t, &job.ObjectMeta, tt.wantJobMeta.Labels, tt.wantJobMeta.Annotations)
			assertObjectMeta(t, &job.Spec.Template.ObjectMeta, tt.wantPodMeta.Labels, tt.wantPodMeta.Annotations)
		})
	}
}

func TestPhysicalBackupRestoreJobImagePullSecrets(t *testing.T) {
	builder := newDefaultTestBuilder(t)
	objMeta := metav1.ObjectMeta{
		Name:      "physical-backup-restore-image-pull-secrets",
		Namespace: "test",
	}
	key := types.NamespacedName{
		Name:      "physical-backup-restore-job",
		Namespace: "test",
	}

	tests := []struct {
		name            string
		mariadb         *mariadbv1alpha1.MariaDB
		wantPullSecrets []corev1.LocalObjectReference
	}{
		{
			name: "No Secrets",
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					BootstrapFrom: &mariadbv1alpha1.BootstrapFrom{
						Volume: &mariadbv1alpha1.StorageVolumeSource{
							EmptyDir: &mariadbv1alpha1.EmptyDirVolumeSource{},
						},
					},
				},
			},
			wantPullSecrets: nil,
		},
		{
			name: "Secrets in MariaDB",
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					BootstrapFrom: &mariadbv1alpha1.BootstrapFrom{
						Volume: &mariadbv1alpha1.StorageVolumeSource{
							EmptyDir: &mariadbv1alpha1.EmptyDirVolumeSource{},
						},
					},
					PodTemplate: mariadbv1alpha1.PodTemplate{
						ImagePullSecrets: []mariadbv1alpha1.LocalObjectReference{
							{
								Name: "mariadb-registry",
							},
						},
					},
				},
			},
			wantPullSecrets: []corev1.LocalObjectReference{
				{
					Name: "mariadb-registry",
				},
			},
		},
	}

	podIndex := ptr.To(0)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job, err := builder.BuildPhysicalBackupRestoreJob(key, tt.mariadb, podIndex)
			if err != nil {
				t.Fatalf("unexpected error building PhysicalBackupRestoreJob: %v", err)
			}
			if !reflect.DeepEqual(tt.wantPullSecrets, job.Spec.Template.Spec.ImagePullSecrets) {
				t.Errorf("unexpected ImagePullSecrets, want: %v  got: %v", tt.wantPullSecrets, job.Spec.Template.Spec.ImagePullSecrets)
			}
		})
	}
}

func TestGaleraInitJobImagePullSecrets(t *testing.T) {
	builder := newDefaultTestBuilder(t)
	objMeta := metav1.ObjectMeta{
		Name: "init-image-pull-secrets",
	}

	tests := []struct {
		name            string
		mariadb         *mariadbv1alpha1.MariaDB
		wantPullSecrets []corev1.LocalObjectReference
	}{
		{
			name: "No Secrets",
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Galera: &mariadbv1alpha1.Galera{
						Enabled: true,
					},
				},
			},
			wantPullSecrets: nil,
		},
		{
			name: "Secrets in MariaDB",
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Galera: &mariadbv1alpha1.Galera{
						Enabled: true,
					},
					PodTemplate: mariadbv1alpha1.PodTemplate{
						ImagePullSecrets: []mariadbv1alpha1.LocalObjectReference{
							{
								Name: "mariadb-registry",
							},
						},
					},
				},
			},
			wantPullSecrets: []corev1.LocalObjectReference{
				{
					Name: "mariadb-registry",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job, err := builder.BuildGaleraInitJob(tt.mariadb.InitKey(), tt.mariadb)
			if err != nil {
				t.Fatalf("unexpected error building Job: %v", err)
			}
			if !reflect.DeepEqual(tt.wantPullSecrets, job.Spec.Template.Spec.ImagePullSecrets) {
				t.Errorf("unexpected ImagePullSecrets, want: %v  got: %v", tt.wantPullSecrets, job.Spec.Template.Spec.ImagePullSecrets)
			}
		})
	}
}

func TestGaleraInitJobMeta(t *testing.T) {
	builder := newDefaultTestBuilder(t)
	key := types.NamespacedName{
		Name: "init-obj",
	}
	mariadbObjMeta := metav1.ObjectMeta{
		Name: "mariadb-obj",
	}
	tests := []struct {
		name        string
		mariadb     *mariadbv1alpha1.MariaDB
		wantJobMeta *mariadbv1alpha1.Metadata
		wantPodMeta *mariadbv1alpha1.Metadata
	}{
		{
			name: "empty",
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: mariadbObjMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Galera: &mariadbv1alpha1.Galera{
						Enabled: true,
					},
				},
			},
			wantJobMeta: &mariadbv1alpha1.Metadata{
				Labels:      map[string]string{},
				Annotations: map[string]string{},
			},
			wantPodMeta: &mariadbv1alpha1.Metadata{
				Labels:      map[string]string{},
				Annotations: map[string]string{},
			},
		},
		{
			name: "inherit meta",
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: mariadbObjMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Galera: &mariadbv1alpha1.Galera{
						Enabled: true,
					},
					InheritMetadata: &mariadbv1alpha1.Metadata{
						Labels: map[string]string{
							"sidecar.istio.io/inject": "false",
						},
						Annotations: map[string]string{
							"database.myorg.io": "mariadb",
						},
					},
				},
			},
			wantJobMeta: &mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"sidecar.istio.io/inject": "false",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
			wantPodMeta: &mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"sidecar.istio.io/inject": "false",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
		},
		{
			name: "extra meta",
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: mariadbObjMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Galera: &mariadbv1alpha1.Galera{
						Enabled: true,
						GaleraSpec: mariadbv1alpha1.GaleraSpec{
							InitJob: &mariadbv1alpha1.GaleraInitJob{
								Metadata: &mariadbv1alpha1.Metadata{
									Labels: map[string]string{
										"sidecar.istio.io/inject": "false",
									},
									Annotations: map[string]string{
										"database.myorg.io": "mariadb",
									},
								},
							},
						},
					},
				},
			},
			wantJobMeta: &mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"sidecar.istio.io/inject": "false",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
			wantPodMeta: &mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"sidecar.istio.io/inject": "false",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
		},
		{
			name: "Pod meta",
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: mariadbObjMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Galera: &mariadbv1alpha1.Galera{
						Enabled: true,
					},
					PodTemplate: mariadbv1alpha1.PodTemplate{
						PodMetadata: &mariadbv1alpha1.Metadata{
							Labels: map[string]string{
								"sidecar.istio.io/inject": "false",
							},
							Annotations: map[string]string{
								"database.myorg.io": "mariadb",
							},
						},
					},
				},
			},
			wantJobMeta: &mariadbv1alpha1.Metadata{
				Labels:      map[string]string{},
				Annotations: map[string]string{},
			},
			wantPodMeta: &mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"sidecar.istio.io/inject": "false",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
		},
		{
			name: "override Pod meta",
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: mariadbObjMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Galera: &mariadbv1alpha1.Galera{
						Enabled: true,
						GaleraSpec: mariadbv1alpha1.GaleraSpec{
							InitJob: &mariadbv1alpha1.GaleraInitJob{
								Metadata: &mariadbv1alpha1.Metadata{
									Labels: map[string]string{
										"sidecar.istio.io/inject": "true",
									},
								},
							},
						},
					},
					PodTemplate: mariadbv1alpha1.PodTemplate{
						PodMetadata: &mariadbv1alpha1.Metadata{
							Labels: map[string]string{
								"sidecar.istio.io/inject": "false",
							},
							Annotations: map[string]string{
								"database.myorg.io": "mariadb",
							},
						},
					},
				},
			},
			wantJobMeta: &mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"sidecar.istio.io/inject": "true",
				},
				Annotations: map[string]string{},
			},
			wantPodMeta: &mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"sidecar.istio.io/inject": "true",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
		},
		{
			name: "all",
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: mariadbObjMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Galera: &mariadbv1alpha1.Galera{
						Enabled: true,
						GaleraSpec: mariadbv1alpha1.GaleraSpec{
							InitJob: &mariadbv1alpha1.GaleraInitJob{
								Metadata: &mariadbv1alpha1.Metadata{
									Annotations: map[string]string{
										"sidecar.istio.io/inject": "false",
									},
								},
							},
						},
					},
					InheritMetadata: &mariadbv1alpha1.Metadata{
						Annotations: map[string]string{
							"database.myorg.io": "mariadb",
						},
					},
					PodTemplate: mariadbv1alpha1.PodTemplate{
						PodMetadata: &mariadbv1alpha1.Metadata{
							Labels: map[string]string{
								"sidecar.istio.io/inject": "false",
							},
						},
					},
				},
			},
			wantJobMeta: &mariadbv1alpha1.Metadata{
				Labels: map[string]string{},
				Annotations: map[string]string{
					"database.myorg.io":       "mariadb",
					"sidecar.istio.io/inject": "false",
				},
			},
			wantPodMeta: &mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"sidecar.istio.io/inject": "false",
				},
				Annotations: map[string]string{
					"database.myorg.io":       "mariadb",
					"sidecar.istio.io/inject": "false",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job, err := builder.BuildGaleraInitJob(key, tt.mariadb)
			if err != nil {
				t.Fatalf("unexpected error building init Job: %v", err)
			}
			assertObjectMeta(t, &job.ObjectMeta, tt.wantJobMeta.Labels, tt.wantJobMeta.Annotations)
			assertObjectMeta(t, &job.Spec.Template.ObjectMeta, tt.wantPodMeta.Labels, tt.wantPodMeta.Annotations)
		})
	}
}

func TestGaleraInitJobResources(t *testing.T) {
	builder := newDefaultTestBuilder(t)
	key := types.NamespacedName{
		Name: "job-obj",
	}
	objMeta := metav1.ObjectMeta{
		Name: "mariadb-obj",
	}
	tests := []struct {
		name          string
		mariadb       *mariadbv1alpha1.MariaDB
		wantResources corev1.ResourceRequirements
	}{
		{
			name: "no resources",
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Galera: &mariadbv1alpha1.Galera{
						Enabled: true,
					},
				},
			},
			wantResources: corev1.ResourceRequirements{},
		},
		{
			name: "mariadb resources",
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Galera: &mariadbv1alpha1.Galera{
						Enabled: true,
					},
					ContainerTemplate: mariadbv1alpha1.ContainerTemplate{
						Resources: &mariadbv1alpha1.ResourceRequirements{
							Requests: corev1.ResourceList{
								"cpu": resource.MustParse("300m"),
							},
						},
					},
				},
			},
			wantResources: corev1.ResourceRequirements{},
		},
		{
			name: "init Job resources",
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Galera: &mariadbv1alpha1.Galera{
						Enabled: true,
						GaleraSpec: mariadbv1alpha1.GaleraSpec{
							InitJob: &mariadbv1alpha1.GaleraInitJob{
								Resources: &mariadbv1alpha1.ResourceRequirements{
									Requests: corev1.ResourceList{
										"cpu": resource.MustParse("100m"),
									},
								},
							},
						},
					},
				},
			},
			wantResources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					"cpu": resource.MustParse("100m"),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job, err := builder.BuildGaleraInitJob(key, tt.mariadb)
			if err != nil {
				t.Fatalf("unexpected error building Galera init Job: %v", err)
			}
			podTpl := job.Spec.Template
			if len(podTpl.Spec.Containers) != 1 {
				t.Error("expecting to have one container")
			}
			resources := podTpl.Spec.Containers[0].Resources
			if !reflect.DeepEqual(resources, tt.wantResources) {
				t.Errorf("unexpected resources, got: %v, expected: %v", resources, tt.wantResources)
			}
		})
	}
}

func TestGaleraRecoveryJobImagePullSecrets(t *testing.T) {
	builder := newDefaultTestBuilder(t)
	key := types.NamespacedName{
		Name: "recovery-obj",
	}
	objMeta := metav1.ObjectMeta{
		Name: "recovery-job-pull-secrets",
	}
	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "mariadb-galera-0",
		},
		Spec: corev1.PodSpec{
			NodeName: "compute-0",
		},
	}
	tests := []struct {
		name            string
		mariadb         *mariadbv1alpha1.MariaDB
		wantPullSecrets []corev1.LocalObjectReference
	}{
		{
			name: "No Secrets",
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Galera: &mariadbv1alpha1.Galera{
						Enabled: true,
						GaleraSpec: mariadbv1alpha1.GaleraSpec{
							Recovery: &mariadbv1alpha1.GaleraRecovery{
								Enabled: true,
							},
						},
					},
				},
			},
			wantPullSecrets: nil,
		},
		{
			name: "Secrets in MariaDB",
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Galera: &mariadbv1alpha1.Galera{
						Enabled: true,
						GaleraSpec: mariadbv1alpha1.GaleraSpec{
							Recovery: &mariadbv1alpha1.GaleraRecovery{
								Enabled: true,
							},
						},
					},
					PodTemplate: mariadbv1alpha1.PodTemplate{
						ImagePullSecrets: []mariadbv1alpha1.LocalObjectReference{
							{
								Name: "mariadb-registry",
							},
						},
					},
				},
			},
			wantPullSecrets: []corev1.LocalObjectReference{
				{
					Name: "mariadb-registry",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job, err := builder.BuildGaleraRecoveryJob(key, tt.mariadb, &pod)
			if err != nil {
				t.Fatalf("unexpected error building Job: %v", err)
			}
			if !reflect.DeepEqual(tt.wantPullSecrets, job.Spec.Template.Spec.ImagePullSecrets) {
				t.Errorf("unexpected ImagePullSecrets, want: %v  got: %v", tt.wantPullSecrets, job.Spec.Template.Spec.ImagePullSecrets)
			}
		})
	}
}

func TestGaleraRecoveryJobMeta(t *testing.T) {
	builder := newDefaultTestBuilder(t)
	key := types.NamespacedName{
		Name: "recovery-obj",
	}
	mariadbObjMeta := metav1.ObjectMeta{
		Name: "mariadb-obj",
	}
	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "mariadb-galera-0",
		},
		Spec: corev1.PodSpec{
			NodeName: "compute-0",
		},
	}
	tests := []struct {
		name        string
		mariadb     *mariadbv1alpha1.MariaDB
		wantJobMeta *mariadbv1alpha1.Metadata
		wantPodMeta *mariadbv1alpha1.Metadata
	}{
		{
			name: "empty",
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: mariadbObjMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Galera: &mariadbv1alpha1.Galera{
						Enabled: true,
						GaleraSpec: mariadbv1alpha1.GaleraSpec{
							Recovery: &mariadbv1alpha1.GaleraRecovery{
								Enabled: true,
							},
						},
					},
				},
			},
			wantJobMeta: &mariadbv1alpha1.Metadata{
				Labels:      map[string]string{},
				Annotations: map[string]string{},
			},
			wantPodMeta: &mariadbv1alpha1.Metadata{
				Labels:      map[string]string{},
				Annotations: map[string]string{},
			},
		},
		{
			name: "inherit meta",
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: mariadbObjMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Galera: &mariadbv1alpha1.Galera{
						Enabled: true,
						GaleraSpec: mariadbv1alpha1.GaleraSpec{
							Recovery: &mariadbv1alpha1.GaleraRecovery{
								Enabled: true,
							},
						},
					},
					InheritMetadata: &mariadbv1alpha1.Metadata{
						Labels: map[string]string{
							"sidecar.istio.io/inject": "false",
						},
						Annotations: map[string]string{
							"database.myorg.io": "mariadb",
						},
					},
				},
			},
			wantJobMeta: &mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"sidecar.istio.io/inject": "false",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
			wantPodMeta: &mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"sidecar.istio.io/inject": "false",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
		},
		{
			name: "extra meta",
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: mariadbObjMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Galera: &mariadbv1alpha1.Galera{
						Enabled: true,
						GaleraSpec: mariadbv1alpha1.GaleraSpec{
							Recovery: &mariadbv1alpha1.GaleraRecovery{
								Enabled: true,
								Job: &mariadbv1alpha1.GaleraRecoveryJob{
									Metadata: &mariadbv1alpha1.Metadata{
										Labels: map[string]string{
											"sidecar.istio.io/inject": "false",
										},
										Annotations: map[string]string{
											"database.myorg.io": "mariadb",
										},
									},
								},
							},
						},
					},
				},
			},
			wantJobMeta: &mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"sidecar.istio.io/inject": "false",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
			wantPodMeta: &mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"sidecar.istio.io/inject": "false",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
		},
		{
			name: "Pod meta",
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: mariadbObjMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Galera: &mariadbv1alpha1.Galera{
						Enabled: true,
						GaleraSpec: mariadbv1alpha1.GaleraSpec{
							Recovery: &mariadbv1alpha1.GaleraRecovery{
								Enabled: true,
							},
						},
					},
					PodTemplate: mariadbv1alpha1.PodTemplate{
						PodMetadata: &mariadbv1alpha1.Metadata{
							Labels: map[string]string{
								"sidecar.istio.io/inject": "false",
							},
							Annotations: map[string]string{
								"database.myorg.io": "mariadb",
							},
						},
					},
				},
			},
			wantJobMeta: &mariadbv1alpha1.Metadata{
				Labels:      map[string]string{},
				Annotations: map[string]string{},
			},
			wantPodMeta: &mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"sidecar.istio.io/inject": "false",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
		},
		{
			name: "override Pod meta",
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: mariadbObjMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Galera: &mariadbv1alpha1.Galera{
						Enabled: true,
						GaleraSpec: mariadbv1alpha1.GaleraSpec{
							Recovery: &mariadbv1alpha1.GaleraRecovery{
								Enabled: true,
								Job: &mariadbv1alpha1.GaleraRecoveryJob{
									Metadata: &mariadbv1alpha1.Metadata{
										Labels: map[string]string{
											"sidecar.istio.io/inject": "true",
										},
									},
								},
							},
						},
					},
					PodTemplate: mariadbv1alpha1.PodTemplate{
						PodMetadata: &mariadbv1alpha1.Metadata{
							Labels: map[string]string{
								"sidecar.istio.io/inject": "false",
							},
							Annotations: map[string]string{
								"database.myorg.io": "mariadb",
							},
						},
					},
				},
			},
			wantJobMeta: &mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"sidecar.istio.io/inject": "true",
				},
				Annotations: map[string]string{},
			},
			wantPodMeta: &mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"sidecar.istio.io/inject": "true",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
		},
		{
			name: "all",
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: mariadbObjMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Galera: &mariadbv1alpha1.Galera{
						Enabled: true,
						GaleraSpec: mariadbv1alpha1.GaleraSpec{
							Recovery: &mariadbv1alpha1.GaleraRecovery{
								Enabled: true,
								Job: &mariadbv1alpha1.GaleraRecoveryJob{
									Metadata: &mariadbv1alpha1.Metadata{
										Annotations: map[string]string{
											"sidecar.istio.io/inject": "false",
										},
									},
								},
							},
						},
					},
					InheritMetadata: &mariadbv1alpha1.Metadata{
						Annotations: map[string]string{
							"database.myorg.io": "mariadb",
						},
					},
					PodTemplate: mariadbv1alpha1.PodTemplate{
						PodMetadata: &mariadbv1alpha1.Metadata{
							Labels: map[string]string{
								"sidecar.istio.io/inject": "false",
							},
						},
					},
				},
			},
			wantJobMeta: &mariadbv1alpha1.Metadata{
				Labels: map[string]string{},
				Annotations: map[string]string{
					"database.myorg.io":       "mariadb",
					"sidecar.istio.io/inject": "false",
				},
			},
			wantPodMeta: &mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"sidecar.istio.io/inject": "false",
				},
				Annotations: map[string]string{
					"database.myorg.io":       "mariadb",
					"sidecar.istio.io/inject": "false",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job, err := builder.BuildGaleraRecoveryJob(key, tt.mariadb, &pod)
			if err != nil {
				t.Fatalf("unexpected error building Galera recovery Job: %v", err)
			}
			assertObjectMeta(t, &job.ObjectMeta, tt.wantJobMeta.Labels, tt.wantJobMeta.Annotations)
			assertObjectMeta(t, &job.Spec.Template.ObjectMeta, tt.wantPodMeta.Labels, tt.wantPodMeta.Annotations)
		})
	}
}

func TestGaleraRecoveryJobVolumes(t *testing.T) {
	builder := newDefaultTestBuilder(t)
	key := types.NamespacedName{
		Name: "job-obj",
	}
	objMeta := metav1.ObjectMeta{
		Name: "mariadb-obj",
	}
	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "mariadb-galera-0",
		},
		Spec: corev1.PodSpec{
			NodeName: "compute-0",
		},
	}
	tests := []struct {
		name        string
		mariadb     *mariadbv1alpha1.MariaDB
		wantVolumes []string
	}{
		{
			name: "dedicated storage",
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Galera: &mariadbv1alpha1.Galera{
						Enabled: true,
						GaleraSpec: mariadbv1alpha1.GaleraSpec{
							Recovery: &mariadbv1alpha1.GaleraRecovery{
								Enabled: true,
							},
							Config: mariadbv1alpha1.GaleraConfig{
								VolumeClaimTemplate: &mariadbv1alpha1.VolumeClaimTemplate{
									PersistentVolumeClaimSpec: mariadbv1alpha1.PersistentVolumeClaimSpec{
										Resources: corev1.VolumeResourceRequirements{
											Requests: corev1.ResourceList{
												"storage": resource.MustParse("1Gi"),
											},
										},
										AccessModes: []corev1.PersistentVolumeAccessMode{
											corev1.ReadWriteOnce,
										},
									},
								},
							},
						},
					},
					Storage: mariadbv1alpha1.Storage{
						Size: ptr.To(resource.MustParse("1Gi")),
						VolumeClaimTemplate: &mariadbv1alpha1.VolumeClaimTemplate{
							PersistentVolumeClaimSpec: mariadbv1alpha1.PersistentVolumeClaimSpec{
								Resources: corev1.VolumeResourceRequirements{
									Requests: corev1.ResourceList{
										"storage": resource.MustParse("1Gi"),
									},
								},
								AccessModes: []corev1.PersistentVolumeAccessMode{
									corev1.ReadWriteOnce,
								},
							},
						},
					},
				},
			},
			wantVolumes: []string{StorageVolume, galeraresources.GaleraConfigVolume},
		},
		{
			name: "reuse storage",
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Galera: &mariadbv1alpha1.Galera{
						Enabled: true,
						GaleraSpec: mariadbv1alpha1.GaleraSpec{
							Recovery: &mariadbv1alpha1.GaleraRecovery{
								Enabled: true,
							},
							Config: mariadbv1alpha1.GaleraConfig{
								ReuseStorageVolume: ptr.To(true),
							},
						},
					},
					Storage: mariadbv1alpha1.Storage{
						Size: ptr.To(resource.MustParse("1Gi")),
						VolumeClaimTemplate: &mariadbv1alpha1.VolumeClaimTemplate{
							PersistentVolumeClaimSpec: mariadbv1alpha1.PersistentVolumeClaimSpec{
								Resources: corev1.VolumeResourceRequirements{
									Requests: corev1.ResourceList{
										"storage": resource.MustParse("1Gi"),
									},
								},
								AccessModes: []corev1.PersistentVolumeAccessMode{
									corev1.ReadWriteOnce,
								},
							},
						},
					},
				},
			},
			wantVolumes: []string{StorageVolume},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job, err := builder.BuildGaleraRecoveryJob(key, tt.mariadb, &pod)
			if err != nil {
				t.Errorf("unexpected error building Galera recovery Job: %v", err)
			}
			for _, wantVolume := range tt.wantVolumes {
				if !hasVolumePVC(job.Spec.Template.Spec.Volumes, wantVolume) {
					t.Errorf("expecting Volume PVC \"%s\", but it was not found", wantVolume)
				}
			}
		})
	}
}

func TestGaleraRecoveryJobNodeSelector(t *testing.T) {
	builder := newDefaultTestBuilder(t)
	key := types.NamespacedName{
		Name: "job-obj",
	}
	objMeta := metav1.ObjectMeta{
		Name: "mariadb-obj",
	}
	tests := []struct {
		name             string
		mariadb          *mariadbv1alpha1.MariaDB
		pod              *corev1.Pod
		wantNodeSelector map[string]string
		wantErr          bool
	}{
		{
			name: "no Pod index",
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Galera: &mariadbv1alpha1.Galera{
						Enabled: true,
						GaleraSpec: mariadbv1alpha1.GaleraSpec{
							Recovery: &mariadbv1alpha1.GaleraRecovery{
								Enabled: true,
								Job: &mariadbv1alpha1.GaleraRecoveryJob{
									PodAffinity: ptr.To(true),
								},
							},
						},
					},
					Storage: mariadbv1alpha1.Storage{
						Size: ptr.To(resource.MustParse("1Gi")),
					},
				},
			},
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foo",
				},
				Spec: corev1.PodSpec{
					NodeName: "compute-0",
				},
			},
			wantNodeSelector: nil,
			wantErr:          true,
		},
		{
			name: "no Node",
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Galera: &mariadbv1alpha1.Galera{
						Enabled: true,
						GaleraSpec: mariadbv1alpha1.GaleraSpec{
							Recovery: &mariadbv1alpha1.GaleraRecovery{
								Enabled: true,
								Job: &mariadbv1alpha1.GaleraRecoveryJob{
									PodAffinity: ptr.To(true),
								},
							},
						},
					},
					Storage: mariadbv1alpha1.Storage{
						Size: ptr.To(resource.MustParse("1Gi")),
					},
				},
			},
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "mariadb-galera-0",
				},
				Spec: corev1.PodSpec{},
			},
			wantNodeSelector: nil,
			wantErr:          true,
		},
		{
			name: "no recovery Job nodeSelector",
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Galera: &mariadbv1alpha1.Galera{
						Enabled: true,
						GaleraSpec: mariadbv1alpha1.GaleraSpec{
							Recovery: &mariadbv1alpha1.GaleraRecovery{
								Enabled: true,
								Job: &mariadbv1alpha1.GaleraRecoveryJob{
									PodAffinity: ptr.To(false),
								},
							},
						},
					},
					Storage: mariadbv1alpha1.Storage{
						Size: ptr.To(resource.MustParse("1Gi")),
					},
				},
			},
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "mariadb-galera-0",
				},
				Spec: corev1.PodSpec{
					NodeName: "compute-0",
				},
			},
			wantNodeSelector: nil,
			wantErr:          false,
		},
		{
			name: "recovery Job nodeSelector",
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Galera: &mariadbv1alpha1.Galera{
						Enabled: true,
						GaleraSpec: mariadbv1alpha1.GaleraSpec{
							Recovery: &mariadbv1alpha1.GaleraRecovery{
								Enabled: true,
								Job: &mariadbv1alpha1.GaleraRecoveryJob{
									PodAffinity: ptr.To(true),
								},
							},
						},
					},
					Storage: mariadbv1alpha1.Storage{
						Size: ptr.To(resource.MustParse("1Gi")),
					},
				},
			},
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "mariadb-galera-0",
				},
				Spec: corev1.PodSpec{
					NodeName: "compute-0",
				},
			},
			wantNodeSelector: map[string]string{
				"kubernetes.io/hostname": "compute-0",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job, err := builder.BuildGaleraRecoveryJob(key, tt.mariadb, tt.pod)
			if tt.wantErr {
				if err == nil {
					t.Error("expect error to have occurred, got nil")
				}
				if job != nil {
					t.Error("expected Job to be nil")
				}
			} else {
				if err != nil {
					t.Errorf("expect error to not have occurred, got: %v", err)
				}
				if !reflect.DeepEqual(tt.wantNodeSelector, job.Spec.Template.Spec.NodeSelector) {
					t.Errorf("unexpected nodeSelector, want: %v got: %v", tt.wantNodeSelector, job.Spec.Template.Spec.NodeSelector)
				}
			}
		})
	}
}

func TestGaleraRecoveryJobResources(t *testing.T) {
	builder := newDefaultTestBuilder(t)
	key := types.NamespacedName{
		Name: "job-obj",
	}
	objMeta := metav1.ObjectMeta{
		Name: "mariadb-obj",
	}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "mariadb-galera-0",
		},
		Spec: corev1.PodSpec{
			NodeName: "compute-0",
		},
	}
	tests := []struct {
		name          string
		mariadb       *mariadbv1alpha1.MariaDB
		wantResources corev1.ResourceRequirements
	}{
		{
			name: "no resources",
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Galera: &mariadbv1alpha1.Galera{
						Enabled: true,
						GaleraSpec: mariadbv1alpha1.GaleraSpec{
							Recovery: &mariadbv1alpha1.GaleraRecovery{
								Enabled: true,
							},
						},
					},
				},
			},
			wantResources: corev1.ResourceRequirements{},
		},
		{
			name: "mariadb resources",
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Galera: &mariadbv1alpha1.Galera{
						Enabled: true,
						GaleraSpec: mariadbv1alpha1.GaleraSpec{
							Recovery: &mariadbv1alpha1.GaleraRecovery{
								Enabled: true,
							},
						},
					},
					ContainerTemplate: mariadbv1alpha1.ContainerTemplate{
						Resources: &mariadbv1alpha1.ResourceRequirements{
							Requests: corev1.ResourceList{
								"cpu": resource.MustParse("300m"),
							},
						},
					},
				},
			},
			wantResources: corev1.ResourceRequirements{},
		},
		{
			name: "recovery Job resources",
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Galera: &mariadbv1alpha1.Galera{
						Enabled: true,
						GaleraSpec: mariadbv1alpha1.GaleraSpec{
							Recovery: &mariadbv1alpha1.GaleraRecovery{
								Enabled: true,
								Job: &mariadbv1alpha1.GaleraRecoveryJob{
									Resources: &mariadbv1alpha1.ResourceRequirements{
										Requests: corev1.ResourceList{
											"cpu": resource.MustParse("100m"),
										},
									},
								},
							},
						},
					},
				},
			},
			wantResources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					"cpu": resource.MustParse("100m"),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job, err := builder.BuildGaleraRecoveryJob(key, tt.mariadb, pod)
			if err != nil {
				t.Fatalf("unexpected error building Galera recovery Job: %v", err)
			}
			podTpl := job.Spec.Template
			if len(podTpl.Spec.Containers) != 1 {
				t.Error("expecting to have one container")
			}
			resources := podTpl.Spec.Containers[0].Resources
			if !reflect.DeepEqual(resources, tt.wantResources) {
				t.Errorf("unexpected resources, got: %v, expected: %v", resources, tt.wantResources)
			}
		})
	}
}

func TestSqlJobImagePullSecrets(t *testing.T) {
	builder := newDefaultTestBuilder(t)
	objMeta := metav1.ObjectMeta{
		Name:      "sqljob-image-pull-secrets",
		Namespace: "test",
	}

	tests := []struct {
		name            string
		sqlJob          *mariadbv1alpha1.SqlJob
		mariadb         *mariadbv1alpha1.MariaDB
		wantPullSecrets []corev1.LocalObjectReference
	}{
		{
			name: "No Secrets",
			sqlJob: &mariadbv1alpha1.SqlJob{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.SqlJobSpec{
					MariaDBRef: mariadbv1alpha1.MariaDBRef{
						ObjectReference: mariadbv1alpha1.ObjectReference{
							Name: objMeta.Name,
						},
					},
					SqlConfigMapKeyRef: &mariadbv1alpha1.ConfigMapKeySelector{
						LocalObjectReference: mariadbv1alpha1.LocalObjectReference{},
					},
				},
			},
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec:       mariadbv1alpha1.MariaDBSpec{},
			},
			wantPullSecrets: nil,
		},
		{
			name: "Secrets in MariaDB",
			sqlJob: &mariadbv1alpha1.SqlJob{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.SqlJobSpec{
					MariaDBRef: mariadbv1alpha1.MariaDBRef{
						ObjectReference: mariadbv1alpha1.ObjectReference{
							Name: objMeta.Name,
						},
					},
					SqlConfigMapKeyRef: &mariadbv1alpha1.ConfigMapKeySelector{
						LocalObjectReference: mariadbv1alpha1.LocalObjectReference{},
					},
				},
			},
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					PodTemplate: mariadbv1alpha1.PodTemplate{
						ImagePullSecrets: []mariadbv1alpha1.LocalObjectReference{
							{
								Name: "mariadb-registry",
							},
						},
					},
				},
			},
			wantPullSecrets: []corev1.LocalObjectReference{
				{
					Name: "mariadb-registry",
				},
			},
		},
		{
			name: "Secrets in SqlJob",
			sqlJob: &mariadbv1alpha1.SqlJob{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.SqlJobSpec{
					JobPodTemplate: mariadbv1alpha1.JobPodTemplate{
						ImagePullSecrets: []mariadbv1alpha1.LocalObjectReference{
							{
								Name: "sqljob-registry",
							},
						},
					},
					MariaDBRef: mariadbv1alpha1.MariaDBRef{
						ObjectReference: mariadbv1alpha1.ObjectReference{
							Name: objMeta.Name,
						},
					},
					SqlConfigMapKeyRef: &mariadbv1alpha1.ConfigMapKeySelector{
						LocalObjectReference: mariadbv1alpha1.LocalObjectReference{},
					},
				},
			},
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec:       mariadbv1alpha1.MariaDBSpec{},
			},
			wantPullSecrets: []corev1.LocalObjectReference{
				{
					Name: "sqljob-registry",
				},
			},
		},
		{
			name: "Secrets in MariaDB and SqlJob",
			sqlJob: &mariadbv1alpha1.SqlJob{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.SqlJobSpec{
					JobPodTemplate: mariadbv1alpha1.JobPodTemplate{
						ImagePullSecrets: []mariadbv1alpha1.LocalObjectReference{
							{
								Name: "sqljob-registry",
							},
						},
					},
					MariaDBRef: mariadbv1alpha1.MariaDBRef{
						ObjectReference: mariadbv1alpha1.ObjectReference{
							Name: objMeta.Name,
						},
					},
					SqlConfigMapKeyRef: &mariadbv1alpha1.ConfigMapKeySelector{
						LocalObjectReference: mariadbv1alpha1.LocalObjectReference{},
					},
				},
			},
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					PodTemplate: mariadbv1alpha1.PodTemplate{
						ImagePullSecrets: []mariadbv1alpha1.LocalObjectReference{
							{
								Name: "mariadb-registry",
							},
						},
					},
				},
			},
			wantPullSecrets: []corev1.LocalObjectReference{
				{
					Name: "mariadb-registry",
				},
				{
					Name: "sqljob-registry",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job, err := builder.BuildSqlJob(client.ObjectKeyFromObject(tt.sqlJob), tt.sqlJob, tt.mariadb)
			if err != nil {
				t.Fatalf("unexpected error building Job: %v", err)
			}
			if !reflect.DeepEqual(tt.wantPullSecrets, job.Spec.Template.Spec.ImagePullSecrets) {
				t.Errorf("unexpected ImagePullSecrets, want: %v  got: %v", tt.wantPullSecrets, job.Spec.Template.Spec.ImagePullSecrets)
			}
		})
	}
}

func TestSqlJobMeta(t *testing.T) {
	builder := newDefaultTestBuilder(t)
	key := types.NamespacedName{
		Name: "sql-job",
	}
	tests := []struct {
		name        string
		sqlJob      *mariadbv1alpha1.SqlJob
		wantJobMeta *mariadbv1alpha1.Metadata
		wantPodMeta *mariadbv1alpha1.Metadata
	}{
		{
			name: "empty",
			sqlJob: &mariadbv1alpha1.SqlJob{
				Spec: mariadbv1alpha1.SqlJobSpec{
					SqlConfigMapKeyRef: &mariadbv1alpha1.ConfigMapKeySelector{},
				},
			},
			wantJobMeta: &mariadbv1alpha1.Metadata{
				Labels:      map[string]string{},
				Annotations: map[string]string{},
			},
			wantPodMeta: &mariadbv1alpha1.Metadata{
				Labels:      map[string]string{},
				Annotations: map[string]string{},
			},
		},
		{
			name: "inherit metadata",
			sqlJob: &mariadbv1alpha1.SqlJob{
				Spec: mariadbv1alpha1.SqlJobSpec{
					SqlConfigMapKeyRef: &mariadbv1alpha1.ConfigMapKeySelector{},
					InheritMetadata: &mariadbv1alpha1.Metadata{
						Labels: map[string]string{
							"sidecar.istio.io/inject": "false",
						},
						Annotations: map[string]string{
							"database.myorg.io": "mariadb",
						},
					},
				},
			},
			wantJobMeta: &mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"sidecar.istio.io/inject": "false",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
			wantPodMeta: &mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"sidecar.istio.io/inject": "false",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
		},
		{
			name: "Pod meta",
			sqlJob: &mariadbv1alpha1.SqlJob{
				Spec: mariadbv1alpha1.SqlJobSpec{
					SqlConfigMapKeyRef: &mariadbv1alpha1.ConfigMapKeySelector{},
					JobPodTemplate: mariadbv1alpha1.JobPodTemplate{
						PodMetadata: &mariadbv1alpha1.Metadata{
							Labels: map[string]string{
								"sidecar.istio.io/inject": "false",
							},
							Annotations: map[string]string{
								"database.myorg.io": "mariadb",
							},
						},
					},
				},
			},
			wantJobMeta: &mariadbv1alpha1.Metadata{
				Labels:      map[string]string{},
				Annotations: map[string]string{},
			},
			wantPodMeta: &mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"sidecar.istio.io/inject": "false",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
		},
		{
			name: "override inherit metadata",
			sqlJob: &mariadbv1alpha1.SqlJob{
				Spec: mariadbv1alpha1.SqlJobSpec{
					SqlConfigMapKeyRef: &mariadbv1alpha1.ConfigMapKeySelector{},
					InheritMetadata: &mariadbv1alpha1.Metadata{
						Labels: map[string]string{
							"sidecar.istio.io/inject": "true",
						},
						Annotations: map[string]string{
							"database.myorg.io": "mariadb",
						},
					},
					JobPodTemplate: mariadbv1alpha1.JobPodTemplate{
						PodMetadata: &mariadbv1alpha1.Metadata{
							Labels: map[string]string{
								"sidecar.istio.io/inject": "false",
							},
							Annotations: map[string]string{
								"database.myorg.io": "mariadb",
							},
						},
					},
				},
			},
			wantJobMeta: &mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"sidecar.istio.io/inject": "true",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
			wantPodMeta: &mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"sidecar.istio.io/inject": "false",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
		},
		{
			name: "all",
			sqlJob: &mariadbv1alpha1.SqlJob{
				Spec: mariadbv1alpha1.SqlJobSpec{
					SqlConfigMapKeyRef: &mariadbv1alpha1.ConfigMapKeySelector{},
					InheritMetadata: &mariadbv1alpha1.Metadata{
						Annotations: map[string]string{
							"database.myorg.io": "mariadb",
						},
					},
					JobPodTemplate: mariadbv1alpha1.JobPodTemplate{
						PodMetadata: &mariadbv1alpha1.Metadata{
							Labels: map[string]string{
								"sidecar.istio.io/inject": "false",
							},
						},
					},
				},
			},
			wantJobMeta: &mariadbv1alpha1.Metadata{
				Labels: map[string]string{},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
			wantPodMeta: &mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"sidecar.istio.io/inject": "false",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job, err := builder.BuildSqlJob(key, tt.sqlJob, &mariadbv1alpha1.MariaDB{})
			if err != nil {
				t.Fatalf("unexpected error building SqlJob Job: %v", err)
			}
			assertObjectMeta(t, &job.ObjectMeta, tt.wantJobMeta.Labels, tt.wantJobMeta.Annotations)
			assertObjectMeta(t, &job.Spec.Template.ObjectMeta, tt.wantPodMeta.Labels, tt.wantPodMeta.Annotations)
		})
	}
}

func hasVolumePVC(volumes []corev1.Volume, volumeName string) bool {
	for _, v := range volumes {
		if v.PersistentVolumeClaim != nil && v.Name == volumeName {
			return true
		}
	}
	return false
}

func getVolumeSource(name string, job *v1.Job) *corev1.VolumeSource {
	for _, volume := range job.Spec.Template.Spec.Volumes {
		if volume.Name == name {
			return &volume.VolumeSource
		}
	}
	return nil
}
