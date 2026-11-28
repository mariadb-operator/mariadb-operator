package binlog

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/environment"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/refresolver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/events"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestShouldArchiveBinlogsPodSelection(t *testing.T) {
	galeraMariadb := func() *mariadbv1alpha1.MariaDB {
		return &mariadbv1alpha1.MariaDB{
			Spec: mariadbv1alpha1.MariaDBSpec{
				Replicas: 3,
				Galera: &mariadbv1alpha1.Galera{
					Enabled: true,
				},
			},
			Status: mariadbv1alpha1.MariaDBStatus{
				Conditions: []metav1.Condition{
					{
						Type:   mariadbv1alpha1.ConditionTypeGaleraConfigured,
						Status: metav1.ConditionTrue,
					},
				},
			},
		}
	}

	tests := []struct {
		name             string
		podName          string
		podArchiverIndex *int
		wantArchive      bool
		wantErr          bool
	}{
		{
			name:             "default index archives in Pod 0",
			podName:          "mariadb-galera-0",
			podArchiverIndex: nil,
			wantArchive:      true,
		},
		{
			name:             "default index does not archive in Pod 1",
			podName:          "mariadb-galera-1",
			podArchiverIndex: nil,
			wantArchive:      false,
		},
		{
			name:             "explicit index archives in the designated Pod",
			podName:          "mariadb-galera-2",
			podArchiverIndex: ptr.To(2),
			wantArchive:      true,
		},
		{
			name:             "explicit index does not archive in Pod 0",
			podName:          "mariadb-galera-0",
			podArchiverIndex: ptr.To(2),
			wantArchive:      false,
		},
		{
			name:             "out of range index errors in Pod 0",
			podName:          "mariadb-galera-0",
			podArchiverIndex: ptr.To(3),
			wantArchive:      false,
			wantErr:          true,
		},
		{
			name:             "out of range index is silent in other Pods",
			podName:          "mariadb-galera-1",
			podArchiverIndex: ptr.To(3),
			wantArchive:      false,
			wantErr:          false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			archiver := &Archiver{
				env: &environment.PodEnvironment{
					PodName: tt.podName,
				},
				logger: logr.Discard(),
			}
			pitr := &mariadbv1alpha1.PointInTimeRecovery{
				Spec: mariadbv1alpha1.PointInTimeRecoverySpec{
					PodArchiverIndex: tt.podArchiverIndex,
				},
			}

			shouldArchive, err := archiver.shouldArchiveBinlogs(galeraMariadb(), pitr)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.wantArchive, shouldArchive)
		})
	}
}

func TestArchiveTickReportsInvalidPodArchiverIndex(t *testing.T) {
	mdb := &mariadbv1alpha1.MariaDB{
		ObjectMeta: metav1.ObjectMeta{Name: "mariadb-galera", Namespace: "default"},
		Spec: mariadbv1alpha1.MariaDBSpec{
			Replicas:               3,
			Galera:                 &mariadbv1alpha1.Galera{Enabled: true},
			PointInTimeRecoveryRef: &mariadbv1alpha1.LocalObjectReference{Name: "pitr"},
		},
	}
	pitr := &mariadbv1alpha1.PointInTimeRecovery{
		ObjectMeta: metav1.ObjectMeta{Name: "pitr", Namespace: "default"},
		Spec:       mariadbv1alpha1.PointInTimeRecoverySpec{PodArchiverIndex: ptr.To(3)},
	}
	scheme := runtime.NewScheme()
	require.NoError(t, mariadbv1alpha1.AddToScheme(scheme))
	k8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(mdb, pitr).
		WithStatusSubresource(&mariadbv1alpha1.MariaDB{}).
		Build()
	recorder := events.NewFakeRecorder(10)
	archiver := &Archiver{
		env: &environment.PodEnvironment{
			PodName:      "mariadb-galera-0",
			PodNamespace: "default",
			MariadbName:  "mariadb-galera",
		},
		client:      k8sClient,
		refResolver: refresolver.New(k8sClient),
		recorder:    recorder,
		logger:      logr.Discard(),
	}

	require.NoError(t, archiver.archiveTick(context.Background()))

	var got mariadbv1alpha1.MariaDB
	require.NoError(t, k8sClient.Get(context.Background(), client.ObjectKeyFromObject(mdb), &got))
	cond := meta.FindStatusCondition(got.Status.Conditions, mariadbv1alpha1.ConditionTypeBinlogsArchived)
	require.NotNil(t, cond)
	assert.Equal(t, metav1.ConditionFalse, cond.Status)
	assert.Contains(t, cond.Message, "invalid spec.podArchiverIndex 3")

	select {
	case event := <-recorder.Events:
		assert.Contains(t, event, mariadbv1alpha1.ReasonBinlogArchivalError)
	default:
		t.Fatal("expected a BinlogArchivalError event")
	}
}
