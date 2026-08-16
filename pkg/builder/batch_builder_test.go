package builder

import (
	"reflect"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	labels "github.com/mariadb-operator/mariadb-operator/v26/pkg/builder/labels"
	builderpki "github.com/mariadb-operator/mariadb-operator/v26/pkg/builder/pki"
	galeraresources "github.com/mariadb-operator/mariadb-operator/v26/pkg/controller/galera/resources"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/replication"
	v1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("BackupJobImagePullSecrets", func() {
	objMeta := metav1.ObjectMeta{
		Name:      "backup-image-pull-secrets",
		Namespace: "test",
	}
	DescribeTable("BuildBackupJob ImagePullSecrets",
		func(backup *mariadbv1alpha1.Backup, mariadb *mariadbv1alpha1.MariaDB, wantPullSecrets []corev1.LocalObjectReference) {
			builder := newDefaultTestBuilder()
			job, err := builder.BuildBackupJob(client.ObjectKeyFromObject(backup), backup, mariadb)
			Expect(err).NotTo(HaveOccurred())
			Expect(job.Spec.Template.Spec.ImagePullSecrets).To(Equal(wantPullSecrets))
		},
		Entry("No Secrets",
			&mariadbv1alpha1.Backup{
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
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec:       mariadbv1alpha1.MariaDBSpec{},
			},
			nil,
		),
		Entry("Secrets in MariaDB",
			&mariadbv1alpha1.Backup{
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
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					MariaDBPodTemplate: mariadbv1alpha1.MariaDBPodTemplate{
						ImagePullSecrets: []mariadbv1alpha1.LocalObjectReference{
							{
								Name: "mariadb-registry",
							},
						},
					},
				},
			},
			[]corev1.LocalObjectReference{
				{
					Name: "mariadb-registry",
				},
			},
		),
		Entry("Secrets in Backup",
			&mariadbv1alpha1.Backup{
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
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec:       mariadbv1alpha1.MariaDBSpec{},
			},
			[]corev1.LocalObjectReference{
				{
					Name: "backup-registry",
				},
			},
		),
		Entry("Secrets in MariaDB and Backup",
			&mariadbv1alpha1.Backup{
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
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					MariaDBPodTemplate: mariadbv1alpha1.MariaDBPodTemplate{
						ImagePullSecrets: []mariadbv1alpha1.LocalObjectReference{
							{
								Name: "mariadb-registry",
							},
						},
					},
				},
			},
			[]corev1.LocalObjectReference{
				{
					Name: "mariadb-registry",
				},
				{
					Name: "backup-registry",
				},
			},
		),
	)
})

var _ = Describe("BackupJobVolumeSource", func() {
	objMeta := metav1.ObjectMeta{
		Name:      "backup-volume-source",
		Namespace: "test",
	}
	mariadb := &mariadbv1alpha1.MariaDB{
		ObjectMeta: objMeta,
		Spec:       mariadbv1alpha1.MariaDBSpec{},
	}
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
		volumeSource := mariadbv1alpha1.StorageVolumeSource{}
		reflect.ValueOf(&volumeSource).Elem().FieldByName(field.Name).Set(storageVolumeSourceValue.FieldByName(field.Name))

		It(field.Name, func() {
			builder := newDefaultTestBuilder()
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
			Expect(err).NotTo(HaveOccurred())

			coreVolumeSourceValue := reflect.ValueOf(*getVolumeSource(batchStorageVolume, job))
			Expect(coreVolumeSourceValue.FieldByName(field.Name).IsNil()).To(BeFalse())
		})
	}
})

var _ = Describe("BackupJobMeta", func() {
	key := types.NamespacedName{
		Name: "backup-job",
	}
	DescribeTable("BuildBackupJob Meta",
		func(backup *mariadbv1alpha1.Backup, wantJobMeta *mariadbv1alpha1.Metadata, wantPodMeta *mariadbv1alpha1.Metadata) {
			builder := newDefaultTestBuilder()
			job, err := builder.BuildBackupJob(key, backup, &mariadbv1alpha1.MariaDB{})
			Expect(err).NotTo(HaveOccurred())
			assertObjectMeta(&job.ObjectMeta, wantJobMeta.Labels, wantJobMeta.Annotations)
			assertObjectMeta(&job.Spec.Template.ObjectMeta, wantPodMeta.Labels, wantPodMeta.Annotations)
		},
		Entry("empty",
			&mariadbv1alpha1.Backup{
				Spec: mariadbv1alpha1.BackupSpec{
					Storage: mariadbv1alpha1.BackupStorage{
						PersistentVolumeClaim: &mariadbv1alpha1.PersistentVolumeClaimSpec{},
					},
				},
			},
			&mariadbv1alpha1.Metadata{
				Labels:      map[string]string{},
				Annotations: map[string]string{},
			},
			&mariadbv1alpha1.Metadata{
				Labels:      map[string]string{},
				Annotations: map[string]string{},
			},
		),
		Entry("inherit metadata",
			&mariadbv1alpha1.Backup{
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
			&mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"sidecar.istio.io/inject": "false",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
			&mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"sidecar.istio.io/inject": "false",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
		),
		Entry("Pod meta",
			&mariadbv1alpha1.Backup{
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
			&mariadbv1alpha1.Metadata{
				Labels:      map[string]string{},
				Annotations: map[string]string{},
			},
			&mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"sidecar.istio.io/inject": "false",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
		),
		Entry("override inherit metadata",
			&mariadbv1alpha1.Backup{
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
			&mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"sidecar.istio.io/inject": "true",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
			&mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"sidecar.istio.io/inject": "false",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
		),
		Entry("all",
			&mariadbv1alpha1.Backup{
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
			&mariadbv1alpha1.Metadata{
				Labels: map[string]string{},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
			&mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"sidecar.istio.io/inject": "false",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
		),
	)
})

var _ = Describe("PhysicalBackupJobPodAffinity", func() {
	podObjMeta := metav1.ObjectMeta{
		Name: "mariadb-0",
	}
	mariadb := &mariadbv1alpha1.MariaDB{
		ObjectMeta: metav1.ObjectMeta{
			Name: "mariadb",
		},
		Spec: mariadbv1alpha1.MariaDBSpec{},
	}
	DescribeTable("BuildPhysicalBackupJob PodAffinity",
		func(backup *mariadbv1alpha1.PhysicalBackup, pod *corev1.Pod, wantErr bool, wantPodAffinity bool) {
			builder := newDefaultTestBuilder()
			job, err := builder.BuildPhysicalBackupJob(
				client.ObjectKeyFromObject(backup),
				backup,
				mariadb,
				pod,
				"backupfile",
			)
			if wantErr {
				Expect(err).To(HaveOccurred())
				return
			}
			Expect(err).NotTo(HaveOccurred())
			affinity := job.Spec.Template.Spec.Affinity
			if wantPodAffinity {
				Expect(affinity).NotTo(BeNil())
				Expect(affinity.PodAffinity).NotTo(BeNil())
				Expect(affinity.PodAffinity.RequiredDuringSchedulingIgnoredDuringExecution).NotTo(BeEmpty())

				term := affinity.PodAffinity.RequiredDuringSchedulingIgnoredDuringExecution[0]
				Expect(term.TopologyKey).To(Equal("kubernetes.io/hostname"))
				Expect(term.LabelSelector.MatchLabels["app.kubernetes.io/instance"]).To(Equal(mariadb.Name))
				Expect(term.LabelSelector.MatchLabels["statefulset.kubernetes.io/pod-name"]).To(Equal(pod.Name))
			} else {
				Expect(affinity == nil || affinity.PodAffinity == nil).To(BeTrue())
			}
		},
		Entry("error when pod nodeName is empty",
			&mariadbv1alpha1.PhysicalBackup{},
			&corev1.Pod{
				ObjectMeta: podObjMeta,
				Spec: corev1.PodSpec{
					NodeName: "",
				},
			},
			true,
			false,
		),
		Entry("podAffinity set when podAffinity is true (default)",
			&mariadbv1alpha1.PhysicalBackup{
				Spec: mariadbv1alpha1.PhysicalBackupSpec{
					Storage: mariadbv1alpha1.PhysicalBackupStorage{
						Volume: &mariadbv1alpha1.StorageVolumeSource{
							EmptyDir: &mariadbv1alpha1.EmptyDirVolumeSource{},
						},
					},
				},
			},
			&corev1.Pod{
				ObjectMeta: podObjMeta,
				Spec: corev1.PodSpec{
					NodeName: "node-1",
				},
			},
			false,
			true,
		),
		Entry("podAffinity set when podAffinity is true (explicit)",
			&mariadbv1alpha1.PhysicalBackup{
				Spec: mariadbv1alpha1.PhysicalBackupSpec{
					PodAffinity: ptr.To(true),
					Storage: mariadbv1alpha1.PhysicalBackupStorage{
						Volume: &mariadbv1alpha1.StorageVolumeSource{
							EmptyDir: &mariadbv1alpha1.EmptyDirVolumeSource{},
						},
					},
				},
			},
			&corev1.Pod{
				ObjectMeta: podObjMeta,
				Spec: corev1.PodSpec{
					NodeName: "node-1",
				},
			},
			false,
			true,
		),
		Entry("podAffinity not set when podAffinity is false",
			&mariadbv1alpha1.PhysicalBackup{
				Spec: mariadbv1alpha1.PhysicalBackupSpec{
					PodAffinity: ptr.To(false),
					Storage: mariadbv1alpha1.PhysicalBackupStorage{
						Volume: &mariadbv1alpha1.StorageVolumeSource{
							EmptyDir: &mariadbv1alpha1.EmptyDirVolumeSource{},
						},
					},
				},
			},
			&corev1.Pod{
				ObjectMeta: podObjMeta,
				Spec: corev1.PodSpec{
					NodeName: "node-1",
				},
			},
			false,
			false,
		),
	)
})

var _ = Describe("PhysicalBackupJobMeta", func() {
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
	DescribeTable("BuildPhysicalBackupJob Meta",
		func(backup *mariadbv1alpha1.PhysicalBackup, wantJobMeta *mariadbv1alpha1.Metadata, wantPodMeta *mariadbv1alpha1.Metadata) {
			builder := newDefaultTestBuilder()
			job, err := builder.BuildPhysicalBackupJob(key, backup, &mariadbv1alpha1.MariaDB{}, pod, "backupfile")
			Expect(err).NotTo(HaveOccurred())
			assertObjectMeta(&job.ObjectMeta, wantJobMeta.Labels, wantJobMeta.Annotations)
			assertObjectMeta(&job.Spec.Template.ObjectMeta, wantPodMeta.Labels, wantPodMeta.Annotations)
		},
		Entry("empty",
			&mariadbv1alpha1.PhysicalBackup{
				Spec: mariadbv1alpha1.PhysicalBackupSpec{
					Storage: mariadbv1alpha1.PhysicalBackupStorage{
						Volume: &mariadbv1alpha1.StorageVolumeSource{
							PersistentVolumeClaim: &mariadbv1alpha1.PersistentVolumeClaimVolumeSource{},
						},
					},
				},
			},
			&mariadbv1alpha1.Metadata{
				Labels:      map[string]string{},
				Annotations: map[string]string{},
			},
			&mariadbv1alpha1.Metadata{
				Labels:      map[string]string{},
				Annotations: map[string]string{},
			},
		),
		Entry("inherit metadata",
			&mariadbv1alpha1.PhysicalBackup{
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
			&mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"sidecar.istio.io/inject": "false",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
			&mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"sidecar.istio.io/inject": "false",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
		),
		Entry("Pod meta",
			&mariadbv1alpha1.PhysicalBackup{
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
			&mariadbv1alpha1.Metadata{
				Labels:      map[string]string{},
				Annotations: map[string]string{},
			},
			&mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"sidecar.istio.io/inject": "false",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
		),
		Entry("override inherit metadata",
			&mariadbv1alpha1.PhysicalBackup{
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
			&mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"sidecar.istio.io/inject": "true",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
			&mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"sidecar.istio.io/inject": "false",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
		),
		Entry("all",
			&mariadbv1alpha1.PhysicalBackup{
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
			&mariadbv1alpha1.Metadata{
				Labels: map[string]string{},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
			&mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"sidecar.istio.io/inject": "false",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
		),
	)
})

var _ = Describe("PhysicalBackupJobImagePullSecrets", func() {
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
	DescribeTable("BuildPhysicalBackupJob ImagePullSecrets",
		func(backup *mariadbv1alpha1.PhysicalBackup, mariadb *mariadbv1alpha1.MariaDB, wantPullSecrets []corev1.LocalObjectReference) {
			builder := newDefaultTestBuilder()
			job, err := builder.BuildPhysicalBackupJob(
				client.ObjectKeyFromObject(backup),
				backup,
				mariadb,
				pod,
				"backupfile",
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(job.Spec.Template.Spec.ImagePullSecrets).To(Equal(wantPullSecrets))
		},
		Entry("No Secrets",
			&mariadbv1alpha1.PhysicalBackup{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.PhysicalBackupSpec{
					Storage: mariadbv1alpha1.PhysicalBackupStorage{
						Volume: &mariadbv1alpha1.StorageVolumeSource{
							EmptyDir: &mariadbv1alpha1.EmptyDirVolumeSource{},
						},
					},
				},
			},
			mariadb,
			nil,
		),
		Entry("Secrets in MariaDB",
			&mariadbv1alpha1.PhysicalBackup{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.PhysicalBackupSpec{
					Storage: mariadbv1alpha1.PhysicalBackupStorage{
						Volume: &mariadbv1alpha1.StorageVolumeSource{
							EmptyDir: &mariadbv1alpha1.EmptyDirVolumeSource{},
						},
					},
				},
			},
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					MariaDBPodTemplate: mariadbv1alpha1.MariaDBPodTemplate{
						ImagePullSecrets: []mariadbv1alpha1.LocalObjectReference{
							{
								Name: "mariadb-registry",
							},
						},
					},
				},
			},
			[]corev1.LocalObjectReference{
				{
					Name: "mariadb-registry",
				},
			},
		),
		Entry("Secrets in PhysicalBackup",
			&mariadbv1alpha1.PhysicalBackup{
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
			mariadb,
			[]corev1.LocalObjectReference{
				{
					Name: "physicalbackup-registry",
				},
			},
		),
		Entry("Secrets in MariaDB and PhysicalBackup",
			&mariadbv1alpha1.PhysicalBackup{
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
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					MariaDBPodTemplate: mariadbv1alpha1.MariaDBPodTemplate{
						ImagePullSecrets: []mariadbv1alpha1.LocalObjectReference{
							{
								Name: "mariadb-registry",
							},
						},
					},
				},
			},
			[]corev1.LocalObjectReference{
				{
					Name: "mariadb-registry",
				},
				{
					Name: "physicalbackup-registry",
				},
			},
		),
	)
})

var _ = Describe("PhysicalBackupJobInitContainers", func() {
	DescribeTable("BuildPhysicalBackupJob InitContainers",
		func(mariadb *mariadbv1alpha1.MariaDB, wantInitContainers []string) {
			builder := newDefaultTestBuilder()

			key := types.NamespacedName{
				Name:      "test-backup",
				Namespace: "test-namespace",
			}
			backup := &mariadbv1alpha1.PhysicalBackup{
				Spec: mariadbv1alpha1.PhysicalBackupSpec{
					Storage: mariadbv1alpha1.PhysicalBackupStorage{
						S3: &mariadbv1alpha1.S3{
							Bucket:   "test",
							Endpoint: "test",
						},
					},
					Compression: mariadbv1alpha1.CompressBzip2,
				},
			}
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "mariadb-0",
				},
				Spec: corev1.PodSpec{
					NodeName: "node1",
				},
			}

			job, err := builder.BuildPhysicalBackupJob(key, backup, mariadb, pod, "backup.xb.bz2")

			Expect(err).NotTo(HaveOccurred())
			Expect(job).NotTo(BeNil())
			Expect(job.Spec.Template.Spec.InitContainers).NotTo(BeNil())
			Expect(job.Spec.Template.Spec.InitContainers).To(HaveLen(len(wantInitContainers)))

			for i, container := range job.Spec.Template.Spec.InitContainers {
				Expect(container.Name).To(Equal(wantInitContainers[i]))
			}
		},
		Entry("Point-in-time recovery disabled",
			&mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Storage: mariadbv1alpha1.Storage{
						Size: ptr.To(resource.MustParse("1Gi")),
					},
				},
			},
			[]string{"mariadb"},
		),
		Entry("Replication and point-in-time recovery enabled",
			&mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					PointInTimeRecoveryRef: &mariadbv1alpha1.LocalObjectReference{
						Name: "test",
					},
					Replication: &mariadbv1alpha1.Replication{
						Enabled: true,
					},
					Storage: mariadbv1alpha1.Storage{
						Size: ptr.To(resource.MustParse("1Gi")),
					},
				},
			},
			[]string{"mariadb", "backup-meta"},
		),
	)
})

var _ = Describe("RestoreJobImagePullSecrets", func() {
	objMeta := metav1.ObjectMeta{
		Name:      "restore-image-pull-secrets",
		Namespace: "test",
	}
	DescribeTable("BuildRestoreJob ImagePullSecrets",
		func(restore *mariadbv1alpha1.Restore, mariadb *mariadbv1alpha1.MariaDB, wantPullSecrets []corev1.LocalObjectReference) {
			builder := newDefaultTestBuilder()
			job, err := builder.BuildRestoreJob(client.ObjectKeyFromObject(restore), restore, mariadb)
			Expect(err).NotTo(HaveOccurred())
			Expect(job.Spec.Template.Spec.ImagePullSecrets).To(Equal(wantPullSecrets))
		},
		Entry("No Secrets",
			&mariadbv1alpha1.Restore{
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
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec:       mariadbv1alpha1.MariaDBSpec{},
			},
			nil,
		),
		Entry("Secrets in MariaDB",
			&mariadbv1alpha1.Restore{
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
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					MariaDBPodTemplate: mariadbv1alpha1.MariaDBPodTemplate{
						ImagePullSecrets: []mariadbv1alpha1.LocalObjectReference{
							{
								Name: "mariadb-registry",
							},
						},
					},
				},
			},
			[]corev1.LocalObjectReference{
				{
					Name: "mariadb-registry",
				},
			},
		),
		Entry("Secrets in Restore",
			&mariadbv1alpha1.Restore{
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
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec:       mariadbv1alpha1.MariaDBSpec{},
			},
			[]corev1.LocalObjectReference{
				{
					Name: "restore-registry",
				},
			},
		),
		Entry("Secrets in MariaDB and Restore",
			&mariadbv1alpha1.Restore{
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
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					MariaDBPodTemplate: mariadbv1alpha1.MariaDBPodTemplate{
						ImagePullSecrets: []mariadbv1alpha1.LocalObjectReference{
							{
								Name: "mariadb-registry",
							},
						},
					},
				},
			},
			[]corev1.LocalObjectReference{
				{
					Name: "mariadb-registry",
				},
				{
					Name: "restore-registry",
				},
			},
		),
	)
})

var _ = Describe("RestoreJobMeta", func() {
	key := types.NamespacedName{
		Name: "restore-job",
	}
	DescribeTable("BuildRestoreJob Meta",
		func(restore *mariadbv1alpha1.Restore, wantJobMeta *mariadbv1alpha1.Metadata, wantPodMeta *mariadbv1alpha1.Metadata) {
			builder := newDefaultTestBuilder()
			job, err := builder.BuildRestoreJob(key, restore, &mariadbv1alpha1.MariaDB{})
			Expect(err).NotTo(HaveOccurred())
			assertObjectMeta(&job.ObjectMeta, wantJobMeta.Labels, wantJobMeta.Annotations)
			assertObjectMeta(&job.Spec.Template.ObjectMeta, wantPodMeta.Labels, wantPodMeta.Annotations)
		},
		Entry("empty",
			&mariadbv1alpha1.Restore{
				Spec: mariadbv1alpha1.RestoreSpec{
					RestoreSource: mariadbv1alpha1.RestoreSource{
						Volume: &mariadbv1alpha1.StorageVolumeSource{
							PersistentVolumeClaim: &mariadbv1alpha1.PersistentVolumeClaimVolumeSource{},
						},
						S3: &mariadbv1alpha1.S3{},
					},
				},
			},
			&mariadbv1alpha1.Metadata{
				Labels:      map[string]string{},
				Annotations: map[string]string{},
			},
			&mariadbv1alpha1.Metadata{
				Labels:      map[string]string{},
				Annotations: map[string]string{},
			},
		),
		Entry("inherit metadata",
			&mariadbv1alpha1.Restore{
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
			&mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"sidecar.istio.io/inject": "false",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
			&mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"sidecar.istio.io/inject": "false",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
		),
		Entry("Pod meta",
			&mariadbv1alpha1.Restore{
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
			&mariadbv1alpha1.Metadata{
				Labels:      map[string]string{},
				Annotations: map[string]string{},
			},
			&mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"sidecar.istio.io/inject": "false",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
		),
		Entry("override inherit metadata",
			&mariadbv1alpha1.Restore{
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
			&mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"sidecar.istio.io/inject": "true",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
			&mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"sidecar.istio.io/inject": "false",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
		),
		Entry("all",
			&mariadbv1alpha1.Restore{
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
			&mariadbv1alpha1.Metadata{
				Labels: map[string]string{},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
			&mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"sidecar.istio.io/inject": "false",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
		),
	)
})

var _ = Describe("PhysicalBackupRestoreJobSelectorLabels", func() {
	It("sets selector labels on the pod template", func() {
		builder := newDefaultTestBuilder()
		key := types.NamespacedName{
			Name:      "physical-backup-restore-job",
			Namespace: "test",
		}

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

		job, err := builder.BuildPhysicalBackupRestoreJob(
			key,
			mariadb,
			podIndex,
			WithBootstrapFrom(mariadb.Spec.BootstrapFrom),
		)
		Expect(err).NotTo(HaveOccurred())

		selectorLabels := labels.NewLabelsBuilder().WithMariaDBSelectorLabels(mariadb).Build()
		for k, v := range selectorLabels {
			Expect(job.Spec.Template.Labels[k]).To(Equal(v))
		}
	})
})

var _ = Describe("PhysicalBackupRestoreJobMeta", func() {
	objMeta := metav1.ObjectMeta{
		Name: "mariadb-obj",
	}
	key := types.NamespacedName{
		Name:      "physical-backup-restore-job-meta",
		Namespace: "test",
	}
	podIndex := ptr.To(0)
	DescribeTable("BuildPhysicalBackupRestoreJob Meta",
		func(mariadb *mariadbv1alpha1.MariaDB, wantJobMeta *mariadbv1alpha1.Metadata, wantPodMeta *mariadbv1alpha1.Metadata) {
			builder := newDefaultTestBuilder()
			job, err := builder.BuildPhysicalBackupRestoreJob(
				key,
				mariadb,
				podIndex,
				WithBootstrapFrom(mariadb.Spec.BootstrapFrom),
			)
			Expect(err).NotTo(HaveOccurred())
			assertObjectMeta(&job.ObjectMeta, wantJobMeta.Labels, wantJobMeta.Annotations)
			assertObjectMeta(&job.Spec.Template.ObjectMeta, wantPodMeta.Labels, wantPodMeta.Annotations)
		},
		Entry("empty",
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					BootstrapFrom: &mariadbv1alpha1.BootstrapFrom{
						Volume: &mariadbv1alpha1.StorageVolumeSource{
							PersistentVolumeClaim: &mariadbv1alpha1.PersistentVolumeClaimVolumeSource{},
						},
					},
				},
			},
			&mariadbv1alpha1.Metadata{
				Labels:      map[string]string{},
				Annotations: map[string]string{},
			},
			&mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"app.kubernetes.io/name":     "mariadb",
					"app.kubernetes.io/instance": "mariadb-obj",
				},
				Annotations: map[string]string{},
			},
		),
		Entry("inherit metadata",
			&mariadbv1alpha1.MariaDB{
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
			&mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"sidecar.istio.io/inject": "false",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
			&mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"sidecar.istio.io/inject":    "false",
					"app.kubernetes.io/name":     "mariadb",
					"app.kubernetes.io/instance": "mariadb-obj",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
		),
		Entry("Pod meta",
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					BootstrapFrom: &mariadbv1alpha1.BootstrapFrom{
						Volume: &mariadbv1alpha1.StorageVolumeSource{
							PersistentVolumeClaim: &mariadbv1alpha1.PersistentVolumeClaimVolumeSource{},
						},
					},
					MariaDBPodTemplate: mariadbv1alpha1.MariaDBPodTemplate{
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
			&mariadbv1alpha1.Metadata{
				Labels:      map[string]string{},
				Annotations: map[string]string{},
			},
			&mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"sidecar.istio.io/inject":    "false",
					"app.kubernetes.io/name":     "mariadb",
					"app.kubernetes.io/instance": "mariadb-obj",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
		),
		Entry("override inherit metadata",
			&mariadbv1alpha1.MariaDB{
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
					MariaDBPodTemplate: mariadbv1alpha1.MariaDBPodTemplate{
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
			&mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"sidecar.istio.io/inject": "true",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
			&mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"sidecar.istio.io/inject":    "false",
					"app.kubernetes.io/name":     "mariadb",
					"app.kubernetes.io/instance": "mariadb-obj",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
		),
		Entry("all",
			&mariadbv1alpha1.MariaDB{
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
					MariaDBPodTemplate: mariadbv1alpha1.MariaDBPodTemplate{
						PodMetadata: &mariadbv1alpha1.Metadata{
							Labels: map[string]string{
								"sidecar.istio.io/inject": "false",
							},
						},
					},
				},
			},
			&mariadbv1alpha1.Metadata{
				Labels: map[string]string{},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
			&mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"sidecar.istio.io/inject":    "false",
					"app.kubernetes.io/name":     "mariadb",
					"app.kubernetes.io/instance": "mariadb-obj",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
		),
	)
})

var _ = Describe("PhysicalBackupRestoreJobImagePullSecrets", func() {
	objMeta := metav1.ObjectMeta{
		Name:      "physical-backup-restore-image-pull-secrets",
		Namespace: "test",
	}
	key := types.NamespacedName{
		Name:      "physical-backup-restore-job",
		Namespace: "test",
	}
	podIndex := ptr.To(0)
	DescribeTable("BuildPhysicalBackupRestoreJob ImagePullSecrets",
		func(mariadb *mariadbv1alpha1.MariaDB, wantPullSecrets []corev1.LocalObjectReference) {
			builder := newDefaultTestBuilder()
			job, err := builder.BuildPhysicalBackupRestoreJob(
				key,
				mariadb,
				podIndex,
				WithBootstrapFrom(mariadb.Spec.BootstrapFrom),
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(job.Spec.Template.Spec.ImagePullSecrets).To(Equal(wantPullSecrets))
		},
		Entry("No Secrets",
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					BootstrapFrom: &mariadbv1alpha1.BootstrapFrom{
						Volume: &mariadbv1alpha1.StorageVolumeSource{
							EmptyDir: &mariadbv1alpha1.EmptyDirVolumeSource{},
						},
					},
				},
			},
			nil,
		),
		Entry("Secrets in MariaDB",
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					BootstrapFrom: &mariadbv1alpha1.BootstrapFrom{
						Volume: &mariadbv1alpha1.StorageVolumeSource{
							EmptyDir: &mariadbv1alpha1.EmptyDirVolumeSource{},
						},
					},
					MariaDBPodTemplate: mariadbv1alpha1.MariaDBPodTemplate{
						ImagePullSecrets: []mariadbv1alpha1.LocalObjectReference{
							{
								Name: "mariadb-registry",
							},
						},
					},
				},
			},
			[]corev1.LocalObjectReference{
				{
					Name: "mariadb-registry",
				},
			},
		),
	)
})

var _ = Describe("GaleraInitJobImagePullSecrets", func() {
	objMeta := metav1.ObjectMeta{
		Name: "init-image-pull-secrets",
	}
	DescribeTable("BuildGaleraInitJob ImagePullSecrets",
		func(mariadb *mariadbv1alpha1.MariaDB, wantPullSecrets []corev1.LocalObjectReference) {
			builder := newDefaultTestBuilder()
			job, err := builder.BuildGaleraInitJob(mariadb.InitKey(), mariadb)
			Expect(err).NotTo(HaveOccurred())
			Expect(job.Spec.Template.Spec.ImagePullSecrets).To(Equal(wantPullSecrets))
		},
		Entry("No Secrets",
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Galera: &mariadbv1alpha1.Galera{
						Enabled: true,
					},
				},
			},
			nil,
		),
		Entry("Secrets in MariaDB",
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Galera: &mariadbv1alpha1.Galera{
						Enabled: true,
					},
					MariaDBPodTemplate: mariadbv1alpha1.MariaDBPodTemplate{
						ImagePullSecrets: []mariadbv1alpha1.LocalObjectReference{
							{
								Name: "mariadb-registry",
							},
						},
					},
				},
			},
			[]corev1.LocalObjectReference{
				{
					Name: "mariadb-registry",
				},
			},
		),
	)
})

var _ = Describe("GaleraInitJobMeta", func() {
	key := types.NamespacedName{
		Name: "init-obj",
	}
	mariadbObjMeta := metav1.ObjectMeta{
		Name: "mariadb-obj",
	}
	DescribeTable("BuildGaleraInitJob Meta",
		func(mariadb *mariadbv1alpha1.MariaDB, wantJobMeta *mariadbv1alpha1.Metadata, wantPodMeta *mariadbv1alpha1.Metadata) {
			builder := newDefaultTestBuilder()
			job, err := builder.BuildGaleraInitJob(key, mariadb)
			Expect(err).NotTo(HaveOccurred())
			assertObjectMeta(&job.ObjectMeta, wantJobMeta.Labels, wantJobMeta.Annotations)
			assertObjectMeta(&job.Spec.Template.ObjectMeta, wantPodMeta.Labels, wantPodMeta.Annotations)
		},
		Entry("empty",
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: mariadbObjMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Galera: &mariadbv1alpha1.Galera{
						Enabled: true,
					},
				},
			},
			&mariadbv1alpha1.Metadata{
				Labels:      map[string]string{},
				Annotations: map[string]string{},
			},
			&mariadbv1alpha1.Metadata{
				Labels:      map[string]string{},
				Annotations: map[string]string{},
			},
		),
		Entry("inherit meta",
			&mariadbv1alpha1.MariaDB{
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
			&mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"sidecar.istio.io/inject": "false",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
			&mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"sidecar.istio.io/inject": "false",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
		),
		Entry("extra meta",
			&mariadbv1alpha1.MariaDB{
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
			&mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"sidecar.istio.io/inject": "false",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
			&mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"sidecar.istio.io/inject": "false",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
		),
		Entry("Pod meta",
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: mariadbObjMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Galera: &mariadbv1alpha1.Galera{
						Enabled: true,
					},
					MariaDBPodTemplate: mariadbv1alpha1.MariaDBPodTemplate{
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
			&mariadbv1alpha1.Metadata{
				Labels:      map[string]string{},
				Annotations: map[string]string{},
			},
			&mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"sidecar.istio.io/inject": "false",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
		),
		Entry("override Pod meta",
			&mariadbv1alpha1.MariaDB{
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
					MariaDBPodTemplate: mariadbv1alpha1.MariaDBPodTemplate{
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
			&mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"sidecar.istio.io/inject": "true",
				},
				Annotations: map[string]string{},
			},
			&mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"sidecar.istio.io/inject": "true",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
		),
		Entry("all",
			&mariadbv1alpha1.MariaDB{
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
					MariaDBPodTemplate: mariadbv1alpha1.MariaDBPodTemplate{
						PodMetadata: &mariadbv1alpha1.Metadata{
							Labels: map[string]string{
								"sidecar.istio.io/inject": "false",
							},
						},
					},
				},
			},
			&mariadbv1alpha1.Metadata{
				Labels: map[string]string{},
				Annotations: map[string]string{
					"database.myorg.io":       "mariadb",
					"sidecar.istio.io/inject": "false",
				},
			},
			&mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"sidecar.istio.io/inject": "false",
				},
				Annotations: map[string]string{
					"database.myorg.io":       "mariadb",
					"sidecar.istio.io/inject": "false",
				},
			},
		),
	)
})

var _ = Describe("GaleraInitJobResources", func() {
	key := types.NamespacedName{
		Name: "job-obj",
	}
	objMeta := metav1.ObjectMeta{
		Name: "mariadb-obj",
	}
	DescribeTable("BuildGaleraInitJob Resources",
		func(mariadb *mariadbv1alpha1.MariaDB, wantResources corev1.ResourceRequirements) {
			builder := newDefaultTestBuilder()
			job, err := builder.BuildGaleraInitJob(key, mariadb)
			Expect(err).NotTo(HaveOccurred())
			podTpl := job.Spec.Template
			Expect(podTpl.Spec.Containers).To(HaveLen(1))
			resources := podTpl.Spec.Containers[0].Resources
			Expect(resources).To(Equal(wantResources))
		},
		Entry("no resources",
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Galera: &mariadbv1alpha1.Galera{
						Enabled: true,
					},
				},
			},
			corev1.ResourceRequirements{},
		),
		Entry("mariadb resources",
			&mariadbv1alpha1.MariaDB{
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
			corev1.ResourceRequirements{},
		),
		Entry("init Job resources",
			&mariadbv1alpha1.MariaDB{
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
			corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					"cpu": resource.MustParse("100m"),
				},
			},
		),
	)
})

var _ = Describe("GaleraInitContainers", func() {
	It("builds the expected init and sidecar containers", func() {
		builder := newDefaultTestBuilder()
		key := types.NamespacedName{
			Name: "job-obj",
		}
		mdb := &mariadbv1alpha1.MariaDB{
			ObjectMeta: metav1.ObjectMeta{
				Name: "mariadb-obj",
			},
			Spec: mariadbv1alpha1.MariaDBSpec{
				MariaDBPodTemplate: mariadbv1alpha1.MariaDBPodTemplate{
					InitContainers: []mariadbv1alpha1.Container{
						{
							Name:    "init",
							Image:   "busybox",
							Command: []string{"bash", "-c"},
							Args:    []string{"exit 0;"},
						},
					},
					SidecarContainers: []mariadbv1alpha1.Container{
						{
							Name:    "sidecar",
							Image:   "busybox",
							Command: []string{"sleep", "infinity"},
						},
					},
				},
				Galera: &mariadbv1alpha1.Galera{
					Enabled: true,
					GaleraSpec: mariadbv1alpha1.GaleraSpec{
						Recovery: &mariadbv1alpha1.GaleraRecovery{
							Enabled: true,
						},
					},
				},
			},
		}

		job, err := builder.BuildGaleraInitJob(key, mdb)
		Expect(err).NotTo(HaveOccurred())
		initContainers := job.Spec.Template.Spec.InitContainers
		containers := job.Spec.Template.Spec.Containers

		Expect(initContainers).To(BeEmpty())
		Expect(containers).To(HaveLen(1))
	})
})

var _ = Describe("GaleraRecoveryJobImagePullSecrets", func() {
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
	DescribeTable("BuildGaleraRecoveryJob ImagePullSecrets",
		func(mariadb *mariadbv1alpha1.MariaDB, wantPullSecrets []corev1.LocalObjectReference) {
			builder := newDefaultTestBuilder()
			job, err := builder.BuildGaleraRecoveryJob(key, mariadb, &pod)
			Expect(err).NotTo(HaveOccurred())
			Expect(job.Spec.Template.Spec.ImagePullSecrets).To(Equal(wantPullSecrets))
		},
		Entry("No Secrets",
			&mariadbv1alpha1.MariaDB{
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
			nil,
		),
		Entry("Secrets in MariaDB",
			&mariadbv1alpha1.MariaDB{
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
					MariaDBPodTemplate: mariadbv1alpha1.MariaDBPodTemplate{
						ImagePullSecrets: []mariadbv1alpha1.LocalObjectReference{
							{
								Name: "mariadb-registry",
							},
						},
					},
				},
			},
			[]corev1.LocalObjectReference{
				{
					Name: "mariadb-registry",
				},
			},
		),
	)
})

var _ = Describe("GaleraRecoveryJobMeta", func() {
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
	DescribeTable("BuildGaleraRecoveryJob Meta",
		func(mariadb *mariadbv1alpha1.MariaDB, wantJobMeta *mariadbv1alpha1.Metadata, wantPodMeta *mariadbv1alpha1.Metadata) {
			builder := newDefaultTestBuilder()
			job, err := builder.BuildGaleraRecoveryJob(key, mariadb, &pod)
			Expect(err).NotTo(HaveOccurred())
			assertObjectMeta(&job.ObjectMeta, wantJobMeta.Labels, wantJobMeta.Annotations)
			assertObjectMeta(&job.Spec.Template.ObjectMeta, wantPodMeta.Labels, wantPodMeta.Annotations)
		},
		Entry("empty",
			&mariadbv1alpha1.MariaDB{
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
			&mariadbv1alpha1.Metadata{
				Labels:      map[string]string{},
				Annotations: map[string]string{},
			},
			&mariadbv1alpha1.Metadata{
				Labels:      map[string]string{},
				Annotations: map[string]string{},
			},
		),
		Entry("inherit meta",
			&mariadbv1alpha1.MariaDB{
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
			&mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"sidecar.istio.io/inject": "false",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
			&mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"sidecar.istio.io/inject": "false",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
		),
		Entry("extra meta",
			&mariadbv1alpha1.MariaDB{
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
			&mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"sidecar.istio.io/inject": "false",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
			&mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"sidecar.istio.io/inject": "false",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
		),
		Entry("Pod meta",
			&mariadbv1alpha1.MariaDB{
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
					MariaDBPodTemplate: mariadbv1alpha1.MariaDBPodTemplate{
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
			&mariadbv1alpha1.Metadata{
				Labels:      map[string]string{},
				Annotations: map[string]string{},
			},
			&mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"sidecar.istio.io/inject": "false",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
		),
		Entry("override Pod meta",
			&mariadbv1alpha1.MariaDB{
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
					MariaDBPodTemplate: mariadbv1alpha1.MariaDBPodTemplate{
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
			&mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"sidecar.istio.io/inject": "true",
				},
				Annotations: map[string]string{},
			},
			&mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"sidecar.istio.io/inject": "true",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
		),
		Entry("all",
			&mariadbv1alpha1.MariaDB{
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
					MariaDBPodTemplate: mariadbv1alpha1.MariaDBPodTemplate{
						PodMetadata: &mariadbv1alpha1.Metadata{
							Labels: map[string]string{
								"sidecar.istio.io/inject": "false",
							},
						},
					},
				},
			},
			&mariadbv1alpha1.Metadata{
				Labels: map[string]string{},
				Annotations: map[string]string{
					"database.myorg.io":       "mariadb",
					"sidecar.istio.io/inject": "false",
				},
			},
			&mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"sidecar.istio.io/inject": "false",
				},
				Annotations: map[string]string{
					"database.myorg.io":       "mariadb",
					"sidecar.istio.io/inject": "false",
				},
			},
		),
	)
})

var _ = Describe("GaleraRecoveryJobVolumes", func() {
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
	DescribeTable("BuildGaleraRecoveryJob Volumes",
		func(mariadb *mariadbv1alpha1.MariaDB, wantVolumes []string) {
			builder := newDefaultTestBuilder()
			job, err := builder.BuildGaleraRecoveryJob(key, mariadb, &pod)
			Expect(err).NotTo(HaveOccurred())
			for _, wantVolume := range wantVolumes {
				Expect(hasVolumePVC(job.Spec.Template.Spec.Volumes, wantVolume)).To(BeTrue())
			}
		},
		Entry("dedicated storage",
			&mariadbv1alpha1.MariaDB{
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
			[]string{StorageVolume, galeraresources.GaleraConfigVolume},
		),
		Entry("reuse storage",
			&mariadbv1alpha1.MariaDB{
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
			[]string{StorageVolume},
		),
	)
})

var _ = Describe("GaleraRecoveryJobNodeSelector", func() {
	key := types.NamespacedName{
		Name: "job-obj",
	}
	objMeta := metav1.ObjectMeta{
		Name: "mariadb-obj",
	}
	DescribeTable("BuildGaleraRecoveryJob NodeSelector",
		func(mariadb *mariadbv1alpha1.MariaDB, pod *corev1.Pod, wantNodeSelector map[string]string, wantErr bool) {
			builder := newDefaultTestBuilder()
			job, err := builder.BuildGaleraRecoveryJob(key, mariadb, pod)
			if wantErr {
				Expect(err).To(HaveOccurred())
				Expect(job).To(BeNil())
			} else {
				Expect(err).NotTo(HaveOccurred())
				Expect(job.Spec.Template.Spec.NodeSelector).To(Equal(wantNodeSelector))
			}
		},
		Entry("no Pod index",
			&mariadbv1alpha1.MariaDB{
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
			&corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foo",
				},
				Spec: corev1.PodSpec{
					NodeName: "compute-0",
				},
			},
			nil,
			true,
		),
		Entry("no Node",
			&mariadbv1alpha1.MariaDB{
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
			&corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "mariadb-galera-0",
				},
				Spec: corev1.PodSpec{},
			},
			nil,
			true,
		),
		Entry("no recovery Job nodeSelector",
			&mariadbv1alpha1.MariaDB{
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
			&corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "mariadb-galera-0",
				},
				Spec: corev1.PodSpec{
					NodeName: "compute-0",
				},
			},
			nil,
			false,
		),
		Entry("recovery Job nodeSelector",
			&mariadbv1alpha1.MariaDB{
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
			&corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "mariadb-galera-0",
				},
				Spec: corev1.PodSpec{
					NodeName: "compute-0",
				},
			},
			map[string]string{
				"kubernetes.io/hostname": "compute-0",
			},
			false,
		),
	)
})

var _ = Describe("GaleraRecoveryJobResources", func() {
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
	DescribeTable("BuildGaleraRecoveryJob Resources",
		func(mariadb *mariadbv1alpha1.MariaDB, wantResources corev1.ResourceRequirements) {
			builder := newDefaultTestBuilder()
			job, err := builder.BuildGaleraRecoveryJob(key, mariadb, pod)
			Expect(err).NotTo(HaveOccurred())
			podTpl := job.Spec.Template
			Expect(podTpl.Spec.Containers).To(HaveLen(1))
			resources := podTpl.Spec.Containers[0].Resources
			Expect(resources).To(Equal(wantResources))
		},
		Entry("no resources",
			&mariadbv1alpha1.MariaDB{
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
			corev1.ResourceRequirements{},
		),
		Entry("mariadb resources",
			&mariadbv1alpha1.MariaDB{
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
			corev1.ResourceRequirements{},
		),
		Entry("recovery Job resources",
			&mariadbv1alpha1.MariaDB{
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
			corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					"cpu": resource.MustParse("100m"),
				},
			},
		),
	)
})

var _ = Describe("GaleraRecoveryContainers", func() {
	It("builds the expected init and sidecar containers", func() {
		builder := newDefaultTestBuilder()
		key := types.NamespacedName{
			Name: "job-obj",
		}
		mdb := &mariadbv1alpha1.MariaDB{
			ObjectMeta: metav1.ObjectMeta{
				Name: "mariadb-obj",
			},
			Spec: mariadbv1alpha1.MariaDBSpec{
				MariaDBPodTemplate: mariadbv1alpha1.MariaDBPodTemplate{
					InitContainers: []mariadbv1alpha1.Container{
						{
							Name:    "init",
							Image:   "busybox",
							Command: []string{"bash", "-c"},
							Args:    []string{"exit 0;"},
						},
					},
					SidecarContainers: []mariadbv1alpha1.Container{
						{
							Name:    "sidecar",
							Image:   "busybox",
							Command: []string{"sleep", "infinity"},
						},
					},
				},
				Galera: &mariadbv1alpha1.Galera{
					Enabled: true,
					GaleraSpec: mariadbv1alpha1.GaleraSpec{
						Recovery: &mariadbv1alpha1.GaleraRecovery{
							Enabled: true,
						},
					},
				},
			},
		}
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name: "mariadb-galera-0",
			},
			Spec: corev1.PodSpec{
				NodeName: "compute-0",
			},
		}

		job, err := builder.BuildGaleraRecoveryJob(key, mdb, pod)
		Expect(err).NotTo(HaveOccurred())
		initContainers := job.Spec.Template.Spec.InitContainers
		containers := job.Spec.Template.Spec.Containers

		Expect(initContainers).To(BeEmpty())
		Expect(containers).To(HaveLen(1))
	})
})

var _ = Describe("GaleraRecoveryJobCommand", func() {
	It("builds the expected galera recovery command", func() {
		expected := "mariadbd --log-error=/dev/stderr --wsrep-recover"
		builder := newDefaultTestBuilder()
		key := types.NamespacedName{
			Name: "job-obj",
		}
		mdb := &mariadbv1alpha1.MariaDB{
			ObjectMeta: metav1.ObjectMeta{
				Name: "mariadb-obj",
			},
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
		}
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name: "mariadb-galera-0",
			},
			Spec: corev1.PodSpec{
				NodeName: "compute-0",
			},
		}

		job, err := builder.BuildGaleraRecoveryJob(key, mdb, pod)
		Expect(err).NotTo(HaveOccurred())

		container := job.Spec.Template.Spec.Containers[0]
		command := strings.Join(append(container.Command, container.Args...), " ")

		Expect(command).To(Equal(expected))
	})
})

var _ = Describe("SqlJobImagePullSecrets", func() {
	objMeta := metav1.ObjectMeta{
		Name:      "sqljob-image-pull-secrets",
		Namespace: "test",
	}
	DescribeTable("BuildSqlJob ImagePullSecrets",
		func(sqlJob *mariadbv1alpha1.SqlJob, mariadb *mariadbv1alpha1.MariaDB, wantPullSecrets []corev1.LocalObjectReference) {
			builder := newDefaultTestBuilder()
			job, err := builder.BuildSqlJob(client.ObjectKeyFromObject(sqlJob), sqlJob, mariadb)
			Expect(err).NotTo(HaveOccurred())
			Expect(job.Spec.Template.Spec.ImagePullSecrets).To(Equal(wantPullSecrets))
		},
		Entry("No Secrets",
			&mariadbv1alpha1.SqlJob{
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
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec:       mariadbv1alpha1.MariaDBSpec{},
			},
			nil,
		),
		Entry("Secrets in MariaDB",
			&mariadbv1alpha1.SqlJob{
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
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					MariaDBPodTemplate: mariadbv1alpha1.MariaDBPodTemplate{
						ImagePullSecrets: []mariadbv1alpha1.LocalObjectReference{
							{
								Name: "mariadb-registry",
							},
						},
					},
				},
			},
			[]corev1.LocalObjectReference{
				{
					Name: "mariadb-registry",
				},
			},
		),
		Entry("Secrets in SqlJob",
			&mariadbv1alpha1.SqlJob{
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
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec:       mariadbv1alpha1.MariaDBSpec{},
			},
			[]corev1.LocalObjectReference{
				{
					Name: "sqljob-registry",
				},
			},
		),
		Entry("Secrets in MariaDB and SqlJob",
			&mariadbv1alpha1.SqlJob{
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
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					MariaDBPodTemplate: mariadbv1alpha1.MariaDBPodTemplate{
						ImagePullSecrets: []mariadbv1alpha1.LocalObjectReference{
							{
								Name: "mariadb-registry",
							},
						},
					},
				},
			},
			[]corev1.LocalObjectReference{
				{
					Name: "mariadb-registry",
				},
				{
					Name: "sqljob-registry",
				},
			},
		),
	)
})

var _ = Describe("SqlJobMeta", func() {
	key := types.NamespacedName{
		Name: "sql-job",
	}
	DescribeTable("BuildSqlJob Meta",
		func(sqlJob *mariadbv1alpha1.SqlJob, wantJobMeta *mariadbv1alpha1.Metadata, wantPodMeta *mariadbv1alpha1.Metadata) {
			builder := newDefaultTestBuilder()
			job, err := builder.BuildSqlJob(key, sqlJob, &mariadbv1alpha1.MariaDB{})
			Expect(err).NotTo(HaveOccurred())
			assertObjectMeta(&job.ObjectMeta, wantJobMeta.Labels, wantJobMeta.Annotations)
			assertObjectMeta(&job.Spec.Template.ObjectMeta, wantPodMeta.Labels, wantPodMeta.Annotations)
		},
		Entry("empty",
			&mariadbv1alpha1.SqlJob{
				Spec: mariadbv1alpha1.SqlJobSpec{
					SqlConfigMapKeyRef: &mariadbv1alpha1.ConfigMapKeySelector{},
				},
			},
			&mariadbv1alpha1.Metadata{
				Labels:      map[string]string{},
				Annotations: map[string]string{},
			},
			&mariadbv1alpha1.Metadata{
				Labels:      map[string]string{},
				Annotations: map[string]string{},
			},
		),
		Entry("inherit metadata",
			&mariadbv1alpha1.SqlJob{
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
			&mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"sidecar.istio.io/inject": "false",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
			&mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"sidecar.istio.io/inject": "false",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
		),
		Entry("Pod meta",
			&mariadbv1alpha1.SqlJob{
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
			&mariadbv1alpha1.Metadata{
				Labels:      map[string]string{},
				Annotations: map[string]string{},
			},
			&mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"sidecar.istio.io/inject": "false",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
		),
		Entry("override inherit metadata",
			&mariadbv1alpha1.SqlJob{
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
			&mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"sidecar.istio.io/inject": "true",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
			&mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"sidecar.istio.io/inject": "false",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
		),
		Entry("all",
			&mariadbv1alpha1.SqlJob{
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
			&mariadbv1alpha1.Metadata{
				Labels: map[string]string{},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
			&mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"sidecar.istio.io/inject": "false",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
		),
	)
})

var _ = Describe("BuildPITRJob", func() {
	pitr := &mariadbv1alpha1.PointInTimeRecovery{
		Spec: mariadbv1alpha1.PointInTimeRecoverySpec{
			PhysicalBackupRef: mariadbv1alpha1.LocalObjectReference{
				Name: "test",
			},
			PointInTimeRecoveryStorage: mariadbv1alpha1.PointInTimeRecoveryStorage{
				S3: &mariadbv1alpha1.S3{
					Bucket:   "test-bucket",
					Endpoint: "s3.amazonaws.com",
					Region:   "us-west-2",
					Prefix:   "test",
				},
			},
		},
	}
	targetRecoveryTime := &metav1.Time{Time: time.Now()}

	DescribeTable("BuildPITRJob",
		func(pitr *mariadbv1alpha1.PointInTimeRecovery, mariadb *mariadbv1alpha1.MariaDB, restoreOptsFn func() []RestoreOpt,
			wantErr bool, wantJobMeta *mariadbv1alpha1.Metadata, wantPodMeta *mariadbv1alpha1.Metadata, wantAffinity bool) {
			b := newDefaultTestBuilder()
			key := types.NamespacedName{
				Name:      "test-pitr-job",
				Namespace: "test",
			}

			job, err := b.BuildPITRJob(key, pitr, mariadb, restoreOptsFn()...)
			if wantErr {
				Expect(err).To(HaveOccurred())
				Expect(job).To(BeNil())
				return
			}
			Expect(err).NotTo(HaveOccurred())
			Expect(job).NotTo(BeNil())

			Expect(job.Name).To(Equal(key.Name))
			Expect(job.Namespace).To(Equal(key.Namespace))

			Expect(job.Spec.Template.Spec.RestartPolicy).To(Equal(corev1.RestartPolicyOnFailure))
			Expect(job.Spec.Template.Spec.Containers).NotTo(BeEmpty())
			Expect(job.Spec.Template.Spec.InitContainers).NotTo(BeEmpty())

			if wantJobMeta != nil {
				assertObjectMeta(&job.ObjectMeta, wantJobMeta.Labels, wantJobMeta.Annotations)
			}
			if wantPodMeta != nil {
				assertObjectMeta(&job.Spec.Template.ObjectMeta, wantPodMeta.Labels, wantPodMeta.Annotations)
			}
			if wantAffinity {
				Expect(job.Spec.Template.Spec.Affinity).NotTo(BeNil())
			} else {
				Expect(job.Spec.Template.Spec.Affinity).To(BeNil())
			}
		},
		Entry("PITR job missing startGtid",
			pitr,
			&mariadbv1alpha1.MariaDB{},
			func() []RestoreOpt {
				return []RestoreOpt{
					WithBootstrapFrom(&mariadbv1alpha1.BootstrapFrom{
						TargetRecoveryTime: targetRecoveryTime,
						Volume: &mariadbv1alpha1.StorageVolumeSource{
							EmptyDir: &mariadbv1alpha1.EmptyDirVolumeSource{},
						},
					}),
				}
			},
			true,
			nil,
			nil,
			false,
		),
		Entry("PITR job missing targetRecoveryTime",
			pitr,
			&mariadbv1alpha1.MariaDB{},
			func() []RestoreOpt {
				return []RestoreOpt{
					WithStartGtid(mustParseGtid("0-10-1")),
				}
			},
			true,
			nil,
			nil,
			false,
		),
		Entry("PITR job missing volume",
			pitr,
			&mariadbv1alpha1.MariaDB{},
			func() []RestoreOpt {
				return []RestoreOpt{
					WithStartGtid(mustParseGtid("0-10-1")),
					WithBootstrapFrom(&mariadbv1alpha1.BootstrapFrom{
						TargetRecoveryTime: &metav1.Time{Time: time.Now()},
					}),
				}
			},
			true,
			nil,
			nil,
			false,
		),
		Entry("PITR job missing volume",
			pitr,
			&mariadbv1alpha1.MariaDB{},
			func() []RestoreOpt {
				return []RestoreOpt{
					WithStartGtid(mustParseGtid("0-10-1")),
					WithBootstrapFrom(&mariadbv1alpha1.BootstrapFrom{
						TargetRecoveryTime: &metav1.Time{Time: time.Now()},
					}),
				}
			},
			true,
			nil,
			nil,
			false,
		),
		Entry("Valid PITR job ",
			pitr,
			&mariadbv1alpha1.MariaDB{},
			func() []RestoreOpt {
				return []RestoreOpt{
					WithStartGtid(mustParseGtid("0-10-1")),
					WithBootstrapFrom(&mariadbv1alpha1.BootstrapFrom{
						TargetRecoveryTime: targetRecoveryTime,
						Volume: &mariadbv1alpha1.StorageVolumeSource{
							EmptyDir: &mariadbv1alpha1.EmptyDirVolumeSource{},
						},
					}),
				}
			},
			false,
			nil,
			nil,
			false,
		),
		Entry("Valid PITR job with meta",
			pitr,
			&mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					InheritMetadata: &mariadbv1alpha1.Metadata{
						Annotations: map[string]string{
							"database.myorg.io": "test",
						},
					},
					MariaDBPodTemplate: mariadbv1alpha1.MariaDBPodTemplate{
						PodMetadata: &mariadbv1alpha1.Metadata{
							Annotations: map[string]string{
								"pod.myorg.io": "test",
							},
						},
					},
				},
			},
			func() []RestoreOpt {
				return []RestoreOpt{
					WithStartGtid(mustParseGtid("0-10-1")),
					WithBootstrapFrom(&mariadbv1alpha1.BootstrapFrom{
						TargetRecoveryTime: targetRecoveryTime,
						Volume: &mariadbv1alpha1.StorageVolumeSource{
							EmptyDir: &mariadbv1alpha1.EmptyDirVolumeSource{},
						},
						RestoreJob: &mariadbv1alpha1.Job{
							Metadata: &mariadbv1alpha1.Metadata{
								Annotations: map[string]string{
									"job.myorg.io": "test",
								},
							},
						},
					}),
				}
			},
			false,
			&mariadbv1alpha1.Metadata{
				Annotations: map[string]string{
					"database.myorg.io": "test",
					"job.myorg.io":      "test",
				},
			},
			&mariadbv1alpha1.Metadata{
				Annotations: map[string]string{
					"database.myorg.io": "test",
					"pod.myorg.io":      "test",
				},
			},
			false,
		),
		Entry("Valid PITR job with affinity",
			pitr,
			&mariadbv1alpha1.MariaDB{},
			func() []RestoreOpt {
				return []RestoreOpt{
					WithStartGtid(mustParseGtid("0-10-1")),
					WithBootstrapFrom(&mariadbv1alpha1.BootstrapFrom{
						TargetRecoveryTime: targetRecoveryTime,
						Volume: &mariadbv1alpha1.StorageVolumeSource{
							EmptyDir: &mariadbv1alpha1.EmptyDirVolumeSource{},
						},
						RestoreJob: &mariadbv1alpha1.Job{
							Affinity: &mariadbv1alpha1.AffinityConfig{
								Affinity: mariadbv1alpha1.Affinity{
									NodeAffinity: &mariadbv1alpha1.NodeAffinity{
										RequiredDuringSchedulingIgnoredDuringExecution: &mariadbv1alpha1.NodeSelector{
											NodeSelectorTerms: []mariadbv1alpha1.NodeSelectorTerm{
												{
													MatchExpressions: []mariadbv1alpha1.NodeSelectorRequirement{
														{
															Key:      "kubernetes.io/hostname",
															Operator: corev1.NodeSelectorOpIn,
															Values:   []string{"node1", "node2"},
														},
													},
												},
											},
										},
									},
								},
							},
						},
					}),
				}
			},
			false,
			nil,
			nil,
			true,
		),
	)
})

var _ = Describe("JobPhysicalBackupVolumes", func() {
	podIndex := 0
	DescribeTable("jobPhysicalBackupVolumes",
		func(storageVolume mariadbv1alpha1.StorageVolumeSource, s3 *mariadbv1alpha1.S3, abs *mariadbv1alpha1.AzureBlob,
			mariadb *mariadbv1alpha1.MariaDB, wantVolumeNames []string) {
			volumes, volumeMounts := jobPhysicalBackupVolumes(storageVolume, s3, abs, mariadb, &podIndex)

			Expect(volumes).To(HaveLen(len(wantVolumeNames)))
			Expect(volumeMounts).To(HaveLen(len(wantVolumeNames)))

			for _, wantName := range wantVolumeNames {
				foundVol := false
				for _, v := range volumes {
					if v.Name == wantName {
						foundVol = true
						break
					}
				}
				Expect(foundVol).To(BeTrue())

				foundMount := false
				for _, vm := range volumeMounts {
					if vm.Name == wantName {
						foundMount = true
						break
					}
				}
				Expect(foundMount).To(BeTrue())
			}
		},
		Entry("Basic backup volumes",
			mariadbv1alpha1.StorageVolumeSource{
				EmptyDir: &mariadbv1alpha1.EmptyDirVolumeSource{},
			},
			(*mariadbv1alpha1.S3)(nil),
			(*mariadbv1alpha1.AzureBlob)(nil),
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: metav1.ObjectMeta{Name: "my-mariadb"},
			},
			[]string{batchStorageVolume, StorageVolume},
		),
		Entry("S3 Volumes",
			mariadbv1alpha1.StorageVolumeSource{},
			&mariadbv1alpha1.S3{
				TLS: &mariadbv1alpha1.TLSConfig{
					Enabled:        true,
					CASecretKeyRef: &mariadbv1alpha1.SecretKeySelector{},
				},
			},
			(*mariadbv1alpha1.AzureBlob)(nil),
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: metav1.ObjectMeta{Name: "my-mariadb"},
			},
			[]string{batchStorageVolume, StorageVolume, S3PKI},
		),
		Entry("ABS Volumes",
			mariadbv1alpha1.StorageVolumeSource{},
			(*mariadbv1alpha1.S3)(nil),
			&mariadbv1alpha1.AzureBlob{
				TLS: &mariadbv1alpha1.TLSConfig{
					Enabled:        true,
					CASecretKeyRef: &mariadbv1alpha1.SecretKeySelector{},
				},
			},
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: metav1.ObjectMeta{Name: "my-mariadb"},
			},
			[]string{batchStorageVolume, StorageVolume, ABSPKI},
		),
		Entry("PKI Volumes",
			mariadbv1alpha1.StorageVolumeSource{},
			(*mariadbv1alpha1.S3)(nil),
			(*mariadbv1alpha1.AzureBlob)(nil),
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: metav1.ObjectMeta{Name: "my-mariadb"},
				Spec: mariadbv1alpha1.MariaDBSpec{
					TLS: &mariadbv1alpha1.TLS{
						Enabled: true,
					},
				},
			},
			[]string{batchStorageVolume, StorageVolume, builderpki.PKIVolume},
		),
		Entry("Additional env",
			mariadbv1alpha1.StorageVolumeSource{},
			(*mariadbv1alpha1.S3)(nil),
			(*mariadbv1alpha1.AzureBlob)(nil),
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: metav1.ObjectMeta{Name: "my-mariadb"},
				Spec: mariadbv1alpha1.MariaDBSpec{
					TLS: &mariadbv1alpha1.TLS{
						Enabled: true,
					},
					ContainerTemplate: mariadbv1alpha1.ContainerTemplate{
						VolumeMounts: []mariadbv1alpha1.VolumeMount{
							{
								Name: "test",
							},
						},
					},
					MariaDBPodTemplate: mariadbv1alpha1.MariaDBPodTemplate{
						Volumes: []mariadbv1alpha1.MariaDBVolume{
							{
								Name: "test",
							},
						},
					},
				},
			},
			[]string{batchStorageVolume, StorageVolume, builderpki.PKIVolume, "test"},
		),
	)
})

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

func mustParseGtid(rawGtid string) *replication.Gtid {
	gtid, err := replication.ParseGtid(rawGtid)
	Expect(err).NotTo(HaveOccurred())
	return gtid
}
