package volumesnapshot

import (
	"time"

	volumesnapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v8/apis/volumesnapshot/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

var now = metav1.Time{Time: time.Now()}

var _ = Describe("IsVolumeSnapshotProvisioned", func() {
	DescribeTable("returns whether the volume snapshot is provisioned",
		func(snap *volumesnapshotv1.VolumeSnapshot, want bool) {
			Expect(IsVolumeSnapshotProvisioned(snap)).To(Equal(want))
		},
		Entry("nil status", &volumesnapshotv1.VolumeSnapshot{}, false),
		Entry("bound empty",
			&volumesnapshotv1.VolumeSnapshot{
				Status: &volumesnapshotv1.VolumeSnapshotStatus{},
			},
			false,
		),
		Entry("bound present but creationTime nil",
			&volumesnapshotv1.VolumeSnapshot{
				Status: &volumesnapshotv1.VolumeSnapshotStatus{
					BoundVolumeSnapshotContentName: ptr.To("bvs-1"),
				},
			},
			false,
		),
		Entry("bound and creationTime present, no error",
			&volumesnapshotv1.VolumeSnapshot{
				Status: &volumesnapshotv1.VolumeSnapshotStatus{
					BoundVolumeSnapshotContentName: ptr.To("bvs-2"),
					CreationTime:                   &now,
				},
			},
			true,
		),
		Entry("bound and creationTime present, but error present",
			&volumesnapshotv1.VolumeSnapshot{
				Status: &volumesnapshotv1.VolumeSnapshotStatus{
					BoundVolumeSnapshotContentName: ptr.To("bvs-3"),
					CreationTime:                   &now,
					Error:                          &volumesnapshotv1.VolumeSnapshotError{Message: ptr.To("boom")},
				},
			},
			false,
		),
	)
})

var _ = Describe("IsVolumeSnapshotReady", func() {
	DescribeTable("returns whether the volume snapshot is ready",
		func(snap *volumesnapshotv1.VolumeSnapshot, want bool) {
			Expect(IsVolumeSnapshotReady(snap)).To(Equal(want))
		},
		Entry("not provisioned", &volumesnapshotv1.VolumeSnapshot{}, false),
		Entry("provisioned but not ready",
			&volumesnapshotv1.VolumeSnapshot{
				Status: &volumesnapshotv1.VolumeSnapshotStatus{
					BoundVolumeSnapshotContentName: ptr.To("bvs-4"),
					CreationTime:                   &now,
					ReadyToUse:                     ptr.To(false),
				},
			},
			false,
		),
		Entry("provisioned and ready",
			&volumesnapshotv1.VolumeSnapshot{
				Status: &volumesnapshotv1.VolumeSnapshotStatus{
					BoundVolumeSnapshotContentName: ptr.To("bvs-5"),
					CreationTime:                   &now,
					ReadyToUse:                     ptr.To(true),
				},
			},
			true,
		),
		Entry("provisioned and ready but error present",
			&volumesnapshotv1.VolumeSnapshot{
				Status: &volumesnapshotv1.VolumeSnapshotStatus{
					BoundVolumeSnapshotContentName: ptr.To("bvs-6"),
					CreationTime:                   &now,
					ReadyToUse:                     ptr.To(true),
					Error:                          &volumesnapshotv1.VolumeSnapshotError{Message: ptr.To("err")},
				},
			},
			false,
		),
	)
})
