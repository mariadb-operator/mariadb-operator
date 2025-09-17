package volumesnapshot

import (
	"testing"
	"time"

	volumesnapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v8/apis/volumesnapshot/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func TestIsVolumeSnapshotProvisioned(t *testing.T) {
	now := metav1.Time{Time: time.Now()}
	cases := []struct {
		name string
		snap *volumesnapshotv1.VolumeSnapshot
		want bool
	}{
		{
			name: "nil status",
			snap: &volumesnapshotv1.VolumeSnapshot{},
			want: false,
		},
		{
			name: "bound empty",
			snap: &volumesnapshotv1.VolumeSnapshot{
				Status: &volumesnapshotv1.VolumeSnapshotStatus{},
			},
			want: false,
		},
		{
			name: "bound present but creationTime nil",
			snap: &volumesnapshotv1.VolumeSnapshot{
				Status: &volumesnapshotv1.VolumeSnapshotStatus{
					BoundVolumeSnapshotContentName: ptr.To("bvs-1"),
				},
			},
			want: false,
		},
		{
			name: "bound and creationTime present, no error",
			snap: &volumesnapshotv1.VolumeSnapshot{
				Status: &volumesnapshotv1.VolumeSnapshotStatus{
					BoundVolumeSnapshotContentName: ptr.To("bvs-2"),
					CreationTime:                   &now,
				},
			},
			want: true,
		},
		{
			name: "bound and creationTime present, but error present",
			snap: &volumesnapshotv1.VolumeSnapshot{
				Status: &volumesnapshotv1.VolumeSnapshotStatus{
					BoundVolumeSnapshotContentName: ptr.To("bvs-3"),
					CreationTime:                   &now,
					Error:                          &volumesnapshotv1.VolumeSnapshotError{Message: ptr.To("boom")},
				},
			},
			want: false,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := IsVolumeSnapshotProvisioned(c.snap)
			if got != c.want {
				t.Fatalf("IsVolumeSnapshotProvisioned(%s) = %v, want %v", c.name, got, c.want)
			}
		})
	}
}

func TestIsVolumeSnapshotReady(t *testing.T) {
	now := metav1.Time{Time: time.Now()}
	cases := []struct {
		name string
		snap *volumesnapshotv1.VolumeSnapshot
		want bool
	}{
		{
			name: "not provisioned",
			snap: &volumesnapshotv1.VolumeSnapshot{},
			want: false,
		},
		{
			name: "provisioned but not ready",
			snap: &volumesnapshotv1.VolumeSnapshot{
				Status: &volumesnapshotv1.VolumeSnapshotStatus{
					BoundVolumeSnapshotContentName: ptr.To("bvs-4"),
					CreationTime:                   &now,
					ReadyToUse:                     ptr.To(false),
				},
			},
			want: false,
		},
		{
			name: "provisioned and ready",
			snap: &volumesnapshotv1.VolumeSnapshot{
				Status: &volumesnapshotv1.VolumeSnapshotStatus{
					BoundVolumeSnapshotContentName: ptr.To("bvs-5"),
					CreationTime:                   &now,
					ReadyToUse:                     ptr.To(true),
				},
			},
			want: true,
		},
		{
			name: "provisioned and ready but error present",
			snap: &volumesnapshotv1.VolumeSnapshot{
				Status: &volumesnapshotv1.VolumeSnapshotStatus{
					BoundVolumeSnapshotContentName: ptr.To("bvs-6"),
					CreationTime:                   &now,
					ReadyToUse:                     ptr.To(true),
					Error:                          &volumesnapshotv1.VolumeSnapshotError{Message: ptr.To("err")},
				},
			},
			want: false,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := IsVolumeSnapshotReady(c.snap)
			if got != c.want {
				t.Fatalf("IsVolumeSnapshotReady(%s) = %v, want %v", c.name, got, c.want)
			}
		})
	}
}
