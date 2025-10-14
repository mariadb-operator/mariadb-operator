package controller

import (
	"time"

	"github.com/go-logr/zapr"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v25/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

var _ = Describe("isRecoverableError", func() {
	logger := zapr.NewLogger(zap.NewNop())

	DescribeTable("should evaluate recoverability",
		func(buildReplicaStatus func() mariadbv1alpha1.ReplicaStatus, mdb *mariadbv1alpha1.MariaDB, expected bool) {
			res := isRecoverableError(mdb, buildReplicaStatus(), recoverableIOErrorCodes, logger)
			Expect(res).To(Equal(expected))
		},
		Entry("recoverable IO code matches",
			func() mariadbv1alpha1.ReplicaStatus {
				return mariadbv1alpha1.ReplicaStatus{
					ReplicaStatusVars: mariadbv1alpha1.ReplicaStatusVars{
						LastIOErrno:  ptr.To(1236),
						LastSQLErrno: nil,
					},
					LastErrorTransitionTime: metav1.Time{},
				}
			},
			&mariadbv1alpha1.MariaDB{},
			true,
		),
		Entry("no errors -> not recoverable",
			func() mariadbv1alpha1.ReplicaStatus {
				return mariadbv1alpha1.ReplicaStatus{
					ReplicaStatusVars: mariadbv1alpha1.ReplicaStatusVars{
						LastIOErrno:  nil,
						LastSQLErrno: nil,
					},
					LastErrorTransitionTime: metav1.Time{},
				}
			},
			&mariadbv1alpha1.MariaDB{},
			false,
		),
		Entry("recent error within threshold -> not recoverable",
			func() mariadbv1alpha1.ReplicaStatus {
				return mariadbv1alpha1.ReplicaStatus{
					ReplicaStatusVars: mariadbv1alpha1.ReplicaStatusVars{
						LastIOErrno:  ptr.To(1),
						LastSQLErrno: ptr.To(0),
					},
					LastErrorTransitionTime: metav1.Now(),
				}
			},
			&mariadbv1alpha1.MariaDB{},
			false,
		),
		Entry("old error older than threshold -> recoverable",
			func() mariadbv1alpha1.ReplicaStatus {
				return mariadbv1alpha1.ReplicaStatus{
					ReplicaStatusVars: mariadbv1alpha1.ReplicaStatusVars{
						LastIOErrno:  ptr.To(1),
						LastSQLErrno: ptr.To(0),
					},
					LastErrorTransitionTime: metav1.NewTime(time.Now().Add(-10 * time.Minute)),
				}
			},
			&mariadbv1alpha1.MariaDB{},
			true,
		),
		Entry("old SQL error older than threshold -> recoverable",
			func() mariadbv1alpha1.ReplicaStatus {
				return mariadbv1alpha1.ReplicaStatus{
					ReplicaStatusVars: mariadbv1alpha1.ReplicaStatusVars{
						LastIOErrno:  ptr.To(1),
						LastSQLErrno: ptr.To(1062),
					},
					LastErrorTransitionTime: metav1.NewTime(time.Now().Add(-10 * time.Minute)),
				}
			},
			&mariadbv1alpha1.MariaDB{},
			true,
		),
		Entry("old SQL error older than custom threshold -> recoverable",
			func() mariadbv1alpha1.ReplicaStatus {
				return mariadbv1alpha1.ReplicaStatus{
					ReplicaStatusVars: mariadbv1alpha1.ReplicaStatusVars{
						LastIOErrno:  ptr.To(1),
						LastSQLErrno: ptr.To(1062),
					},
					LastErrorTransitionTime: metav1.NewTime(time.Now().Add(-1 * time.Minute)),
				}
			},
			&mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Replication: &mariadbv1alpha1.Replication{
						Enabled: true,
						ReplicationSpec: mariadbv1alpha1.ReplicationSpec{
							Replica: mariadbv1alpha1.ReplicaReplication{
								ReplicaRecovery: &mariadbv1alpha1.ReplicaRecovery{
									Enabled:                true,
									ErrorDurationThreshold: &metav1.Duration{Duration: 30 * time.Second},
								},
							},
						},
					},
				},
			},
			true,
		),
	)
})
