package builder

import (
	"reflect"
	"testing"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	galeraresources "github.com/mariadb-operator/mariadb-operator/pkg/controller/galera/resources"
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
						ObjectReference: corev1.ObjectReference{
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
						ObjectReference: corev1.ObjectReference{
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
						ImagePullSecrets: []corev1.LocalObjectReference{
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
						ImagePullSecrets: []corev1.LocalObjectReference{
							{
								Name: "backup-registry",
							},
						},
					},
					MariaDBRef: mariadbv1alpha1.MariaDBRef{
						ObjectReference: corev1.ObjectReference{
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
						ImagePullSecrets: []corev1.LocalObjectReference{
							{
								Name: "backup-registry",
							},
						},
					},
					MariaDBRef: mariadbv1alpha1.MariaDBRef{
						ObjectReference: corev1.ObjectReference{
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
						ImagePullSecrets: []corev1.LocalObjectReference{
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
						ObjectReference: corev1.ObjectReference{
							Name: objMeta.Name,
						},
					},
					RestoreSource: mariadbv1alpha1.RestoreSource{
						Volume: &corev1.VolumeSource{},
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
						ObjectReference: corev1.ObjectReference{
							Name: objMeta.Name,
						},
					},
					RestoreSource: mariadbv1alpha1.RestoreSource{
						Volume: &corev1.VolumeSource{},
					},
				},
			},
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					PodTemplate: mariadbv1alpha1.PodTemplate{
						ImagePullSecrets: []corev1.LocalObjectReference{
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
						ImagePullSecrets: []corev1.LocalObjectReference{
							{
								Name: "restore-registry",
							},
						},
					},
					RestoreSource: mariadbv1alpha1.RestoreSource{
						Volume: &corev1.VolumeSource{},
					},
					MariaDBRef: mariadbv1alpha1.MariaDBRef{
						ObjectReference: corev1.ObjectReference{
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
						ImagePullSecrets: []corev1.LocalObjectReference{
							{
								Name: "restore-registry",
							},
						},
					},
					RestoreSource: mariadbv1alpha1.RestoreSource{
						Volume: &corev1.VolumeSource{},
					},
					MariaDBRef: mariadbv1alpha1.MariaDBRef{
						ObjectReference: corev1.ObjectReference{
							Name: objMeta.Name,
						},
					},
				},
			},
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					PodTemplate: mariadbv1alpha1.PodTemplate{
						ImagePullSecrets: []corev1.LocalObjectReference{
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

func TestInitJobImagePullSecrets(t *testing.T) {
	builder := newDefaultTestBuilder(t)
	objMeta := metav1.ObjectMeta{
		Name:      "init-image-pull-secrets",
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
				Spec:       mariadbv1alpha1.MariaDBSpec{},
			},
			wantPullSecrets: nil,
		},
		{
			name: "Secrets in MariaDB",
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					PodTemplate: mariadbv1alpha1.PodTemplate{
						ImagePullSecrets: []corev1.LocalObjectReference{
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
			job, err := builder.BuilGaleraInitJob(tt.mariadb.InitKey(), tt.mariadb, nil)
			if err != nil {
				t.Fatalf("unexpected error building Job: %v", err)
			}
			if !reflect.DeepEqual(tt.wantPullSecrets, job.Spec.Template.Spec.ImagePullSecrets) {
				t.Errorf("unexpected ImagePullSecrets, want: %v  got: %v", tt.wantPullSecrets, job.Spec.Template.Spec.ImagePullSecrets)
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
						ObjectReference: corev1.ObjectReference{
							Name: objMeta.Name,
						},
					},
					SqlConfigMapKeyRef: &corev1.ConfigMapKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{},
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
						ObjectReference: corev1.ObjectReference{
							Name: objMeta.Name,
						},
					},
					SqlConfigMapKeyRef: &corev1.ConfigMapKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{},
					},
				},
			},
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					PodTemplate: mariadbv1alpha1.PodTemplate{
						ImagePullSecrets: []corev1.LocalObjectReference{
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
						ImagePullSecrets: []corev1.LocalObjectReference{
							{
								Name: "sqljob-registry",
							},
						},
					},
					MariaDBRef: mariadbv1alpha1.MariaDBRef{
						ObjectReference: corev1.ObjectReference{
							Name: objMeta.Name,
						},
					},
					SqlConfigMapKeyRef: &corev1.ConfigMapKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{},
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
						ImagePullSecrets: []corev1.LocalObjectReference{
							{
								Name: "sqljob-registry",
							},
						},
					},
					MariaDBRef: mariadbv1alpha1.MariaDBRef{
						ObjectReference: corev1.ObjectReference{
							Name: objMeta.Name,
						},
					},
					SqlConfigMapKeyRef: &corev1.ConfigMapKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{},
					},
				},
			},
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					PodTemplate: mariadbv1alpha1.PodTemplate{
						ImagePullSecrets: []corev1.LocalObjectReference{
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
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimSpec{},
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
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimSpec{},
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
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimSpec{},
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
			name: "override interit metadata",
			backup: &mariadbv1alpha1.Backup{
				Spec: mariadbv1alpha1.BackupSpec{
					Storage: mariadbv1alpha1.BackupStorage{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimSpec{},
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
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimSpec{},
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
						Volume: &corev1.VolumeSource{
							PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{},
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
						Volume: &corev1.VolumeSource{
							PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{},
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
						Volume: &corev1.VolumeSource{
							PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{},
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
						Volume: &corev1.VolumeSource{
							PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{},
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
						Volume: &corev1.VolumeSource{
							PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{},
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
		initJob     *mariadbv1alpha1.GaleraInitJob
		wantJobMeta *mariadbv1alpha1.Metadata
		wantPodMeta *mariadbv1alpha1.Metadata
	}{
		{
			name: "empty",
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: mariadbObjMeta,
			},
			initJob: &mariadbv1alpha1.GaleraInitJob{},
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
			initJob: &mariadbv1alpha1.GaleraInitJob{},
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
			},
			initJob: &mariadbv1alpha1.GaleraInitJob{
				Job: mariadbv1alpha1.Job{
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
			initJob: &mariadbv1alpha1.GaleraInitJob{},
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
			initJob: &mariadbv1alpha1.GaleraInitJob{
				Job: mariadbv1alpha1.Job{
					Metadata: &mariadbv1alpha1.Metadata{
						Labels: map[string]string{
							"sidecar.istio.io/inject": "true",
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
			initJob: &mariadbv1alpha1.GaleraInitJob{
				Job: mariadbv1alpha1.Job{
					Metadata: &mariadbv1alpha1.Metadata{
						Annotations: map[string]string{
							"sidecar.istio.io/inject": "false",
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
			job, err := builder.BuilGaleraInitJob(key, tt.mariadb, tt.initJob)
			if err != nil {
				t.Fatalf("unexpected error building init Job: %v", err)
			}
			assertObjectMeta(t, &job.ObjectMeta, tt.wantJobMeta.Labels, tt.wantJobMeta.Annotations)
			assertObjectMeta(t, &job.Spec.Template.ObjectMeta, tt.wantPodMeta.Labels, tt.wantPodMeta.Annotations)
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
	tests := []struct {
		name        string
		mariadb     *mariadbv1alpha1.MariaDB
		recoveryJob *mariadbv1alpha1.GaleraRecoveryJob
		wantJobMeta *mariadbv1alpha1.Metadata
		wantPodMeta *mariadbv1alpha1.Metadata
	}{
		{
			name: "empty",
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: mariadbObjMeta,
			},
			recoveryJob: &mariadbv1alpha1.GaleraRecoveryJob{},
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
			recoveryJob: &mariadbv1alpha1.GaleraRecoveryJob{},
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
			},
			recoveryJob: &mariadbv1alpha1.GaleraRecoveryJob{
				Metadata: &mariadbv1alpha1.Metadata{
					Labels: map[string]string{
						"sidecar.istio.io/inject": "false",
					},
					Annotations: map[string]string{
						"database.myorg.io": "mariadb",
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
			recoveryJob: &mariadbv1alpha1.GaleraRecoveryJob{},
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
			recoveryJob: &mariadbv1alpha1.GaleraRecoveryJob{
				Metadata: &mariadbv1alpha1.Metadata{
					Labels: map[string]string{
						"sidecar.istio.io/inject": "true",
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
			recoveryJob: &mariadbv1alpha1.GaleraRecoveryJob{
				Metadata: &mariadbv1alpha1.Metadata{
					Annotations: map[string]string{
						"sidecar.istio.io/inject": "false",
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
			job, err := builder.BuildGaleraRecoveryJob(key, tt.mariadb, tt.recoveryJob, 0)
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
	tests := []struct {
		name        string
		mariadb     *mariadbv1alpha1.MariaDB
		wantVolumes []string
	}{
		{
			name: "galera",
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Storage: mariadbv1alpha1.Storage{
						Size: ptr.To(resource.MustParse("1Gi")),
						VolumeClaimTemplate: &mariadbv1alpha1.VolumeClaimTemplate{
							PersistentVolumeClaimSpec: corev1.PersistentVolumeClaimSpec{
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
					Galera: &mariadbv1alpha1.Galera{
						Enabled: true,
						GaleraSpec: mariadbv1alpha1.GaleraSpec{
							Config: mariadbv1alpha1.GaleraConfig{
								VolumeClaimTemplate: &mariadbv1alpha1.VolumeClaimTemplate{
									PersistentVolumeClaimSpec: corev1.PersistentVolumeClaimSpec{
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
				},
			},
			wantVolumes: []string{StorageVolume, galeraresources.GaleraConfigVolume},
		},
		{
			name: "galera resuse storage",
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Storage: mariadbv1alpha1.Storage{
						Size: ptr.To(resource.MustParse("1Gi")),
						VolumeClaimTemplate: &mariadbv1alpha1.VolumeClaimTemplate{
							PersistentVolumeClaimSpec: corev1.PersistentVolumeClaimSpec{
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
					Galera: &mariadbv1alpha1.Galera{
						Enabled: true,
						GaleraSpec: mariadbv1alpha1.GaleraSpec{
							Config: mariadbv1alpha1.GaleraConfig{
								ReuseStorageVolume: ptr.To(true),
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
			job, err := builder.BuildGaleraRecoveryJob(key, tt.mariadb, nil, 0)
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

func TestGaleraRecoveryJobAffinity(t *testing.T) {
	builder := newDefaultTestBuilder(t)
	key := types.NamespacedName{
		Name: "job-obj",
	}
	objMeta := metav1.ObjectMeta{
		Name: "mariadb-obj",
	}
	tests := []struct {
		name    string
		mariadb *mariadbv1alpha1.MariaDB
	}{
		{
			name: "no mariadb affinity",
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Storage: mariadbv1alpha1.Storage{
						Size: ptr.To(resource.MustParse("1Gi")),
					},
					Galera: &mariadbv1alpha1.Galera{
						Enabled: true,
					},
				},
			},
		},
		{
			name: "mariadb affinity",
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					PodTemplate: mariadbv1alpha1.PodTemplate{
						Affinity: &mariadbv1alpha1.AffinityConfig{
							AntiAffinityEnabled: ptr.To(true),
						},
						TopologySpreadConstraints: []corev1.TopologySpreadConstraint{
							{
								MaxSkew:     1,
								TopologyKey: "kubernetes.io/hostname",
							},
						},
					},
					Storage: mariadbv1alpha1.Storage{
						Size: ptr.To(resource.MustParse("1Gi")),
					},
					Galera: &mariadbv1alpha1.Galera{
						Enabled: true,
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job, err := builder.BuildGaleraRecoveryJob(key, tt.mariadb, nil, 0)
			if err != nil {
				t.Errorf("unexpected error building Galera recovery Job: %v", err)
			}
			if job.Spec.Template.Spec.Affinity != nil {
				t.Error("expected Galera recovery Job to not have affinity")
			}
			if job.Spec.Template.Spec.TopologySpreadConstraints != nil {
				t.Error("expected Galera recovery Job to not have topologySpreadConstraints")
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
					SqlConfigMapKeyRef: &corev1.ConfigMapKeySelector{},
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
					SqlConfigMapKeyRef: &corev1.ConfigMapKeySelector{},
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
					SqlConfigMapKeyRef: &corev1.ConfigMapKeySelector{},
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
			name: "override interit metadata",
			sqlJob: &mariadbv1alpha1.SqlJob{
				Spec: mariadbv1alpha1.SqlJobSpec{
					SqlConfigMapKeyRef: &corev1.ConfigMapKeySelector{},
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
					SqlConfigMapKeyRef: &corev1.ConfigMapKeySelector{},
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
