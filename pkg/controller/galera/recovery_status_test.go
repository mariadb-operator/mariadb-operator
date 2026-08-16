package galera

import (
	"time"

	"github.com/go-logr/logr"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/galera/recovery"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("RecoveryStatus get and set", func() {
	It("gets and sets state, recovered and recovery status", func() {
		rs := newRecoveryStatus(&mariadbv1alpha1.MariaDB{})

		state0 := &recovery.GaleraState{
			Version:         "2.1",
			UUID:            "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
			Seqno:           3,
			SafeToBootstrap: false,
		}
		state1 := &recovery.GaleraState{
			Version:         "2.1",
			UUID:            "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
			Seqno:           6,
			SafeToBootstrap: true,
		}
		state2 := &recovery.GaleraState{
			Version:         "2.1",
			UUID:            "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
			Seqno:           1,
			SafeToBootstrap: true,
		}
		rs.setState("mariadb-galera-0", state0)
		rs.setState("mariadb-galera-1", state1)
		rs.setState("mariadb-galera-2", state2)

		gotState, ok := rs.state("mariadb-galera-1")
		Expect(ok).To(BeTrue())
		Expect(gotState).To(Equal(state1))
		gotState, ok = rs.state("foo-0")
		Expect(ok).To(BeFalse())
		Expect(gotState).To(BeNil())

		recovered0 := &recovery.Bootstrap{
			UUID:  "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
			Seqno: 2,
		}
		recovered1 := &recovery.Bootstrap{
			UUID:  "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
			Seqno: 3,
		}
		recovered2 := &recovery.Bootstrap{
			UUID:  "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
			Seqno: 9,
		}
		rs.setRecovered("mariadb-galera-0", recovered0)
		rs.setRecovered("mariadb-galera-1", recovered1)
		rs.setRecovered("mariadb-galera-2", recovered2)

		gotRecovered, ok := rs.recovered("mariadb-galera-0")
		Expect(ok).To(BeTrue())
		Expect(gotRecovered).To(Equal(recovered0))
		_, ok = rs.recovered("foo-1")
		Expect(ok).To(BeFalse())
		Expect(gotState).To(BeNil())

		expectedRecoveryStatus := mariadbv1alpha1.GaleraRecoveryStatus{
			State: map[string]*recovery.GaleraState{
				"mariadb-galera-0": state0,
				"mariadb-galera-1": state1,
				"mariadb-galera-2": state2,
			},
			Recovered: map[string]*recovery.Bootstrap{
				"mariadb-galera-0": recovered0,
				"mariadb-galera-1": recovered1,
				"mariadb-galera-2": recovered2,
			},
		}
		gotRecoveryStatus := rs.galeraRecoveryStatus()
		Expect(gotRecoveryStatus).To(Equal(expectedRecoveryStatus))

		rs.setPodsRestarted(true)
		expectedRecoveryStatus = mariadbv1alpha1.GaleraRecoveryStatus{
			State: map[string]*recovery.GaleraState{
				"mariadb-galera-0": state0,
				"mariadb-galera-1": state1,
				"mariadb-galera-2": state2,
			},
			Recovered: map[string]*recovery.Bootstrap{
				"mariadb-galera-0": recovered0,
				"mariadb-galera-1": recovered1,
				"mariadb-galera-2": recovered2,
			},
			PodsRestarted: ptr.To(true),
		}

		gotRecoveryStatus = rs.galeraRecoveryStatus()
		Expect(gotRecoveryStatus).To(Equal(expectedRecoveryStatus))
	})
})

var _ = Describe("RecoveryStatus isComplete", func() {
	objMeta := metav1.ObjectMeta{
		Name: "mariadb-galera",
	}

	DescribeTable("returns whether the recovery is complete",
		func(mdb *mariadbv1alpha1.MariaDB, wantBool bool) {
			rs := newRecoveryStatus(mdb)
			complete := rs.isComplete(mdb, logr.Logger{})
			Expect(complete).To(Equal(wantBool))
		},
		Entry("no status",
			&mariadbv1alpha1.MariaDB{},
			false,
		),
		Entry("missing pods",
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Replicas: 3,
				},
				Status: mariadbv1alpha1.MariaDBStatus{
					GaleraRecovery: &mariadbv1alpha1.GaleraRecoveryStatus{
						State: map[string]*recovery.GaleraState{
							"mariadb-galera-0": {
								Version:         "2.1",
								UUID:            "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
								Seqno:           1,
								SafeToBootstrap: false,
							},
							"mariadb-galera-1": {
								Version:         "2.1",
								UUID:            "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
								Seqno:           1,
								SafeToBootstrap: false,
							},
						},
						Recovered: map[string]*recovery.Bootstrap{
							"mariadb-galera-0": {
								UUID:  "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
								Seqno: 1,
							},
							"mariadb-galera-1": {
								UUID:  "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
								Seqno: 1,
							},
						},
					},
				},
			},
			false,
		),
		Entry("safe to bootstrap",
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Replicas: 3,
				},
				Status: mariadbv1alpha1.MariaDBStatus{
					GaleraRecovery: &mariadbv1alpha1.GaleraRecoveryStatus{
						State: map[string]*recovery.GaleraState{
							"mariadb-galera-0": {
								Version:         "2.1",
								UUID:            "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
								Seqno:           1,
								SafeToBootstrap: false,
							},
							"mariadb-galera-1": {
								Version:         "2.1",
								UUID:            "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
								Seqno:           1,
								SafeToBootstrap: false,
							},
							"mariadb-galera-2": {
								Version:         "2.1",
								UUID:            "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
								Seqno:           1,
								SafeToBootstrap: true,
							},
						},
					},
				},
			},
			true,
		),
		Entry("safe to bootstrap with negative seqnos",
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Replicas: 3,
				},
				Status: mariadbv1alpha1.MariaDBStatus{
					GaleraRecovery: &mariadbv1alpha1.GaleraRecoveryStatus{
						State: map[string]*recovery.GaleraState{
							"mariadb-galera-0": {
								Version:         "2.1",
								UUID:            "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
								Seqno:           -1,
								SafeToBootstrap: false,
							},
							"mariadb-galera-1": {
								Version:         "2.1",
								UUID:            "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
								Seqno:           -1,
								SafeToBootstrap: false,
							},
							"mariadb-galera-2": {
								Version:         "2.1",
								UUID:            "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
								Seqno:           1,
								SafeToBootstrap: true,
							},
						},
					},
				},
			},
			true,
		),
		Entry("safe to bootstrap with skipped Pods",
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Replicas: 3,
				},
				Status: mariadbv1alpha1.MariaDBStatus{
					GaleraRecovery: &mariadbv1alpha1.GaleraRecoveryStatus{
						State: map[string]*recovery.GaleraState{
							"mariadb-galera-0": {
								Version:         "2.1",
								UUID:            "00000000-0000-0000-0000-000000000000",
								Seqno:           -1,
								SafeToBootstrap: false,
							},
							"mariadb-galera-1": {
								Version:         "2.1",
								UUID:            "00000000-0000-0000-0000-000000000000",
								Seqno:           -1,
								SafeToBootstrap: false,
							},
							"mariadb-galera-2": {
								Version:         "2.1",
								UUID:            "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
								Seqno:           1,
								SafeToBootstrap: true,
							},
						},
						Recovered: map[string]*recovery.Bootstrap{
							"mariadb-galera-0": {
								UUID:  "00000000-0000-0000-0000-000000000000",
								Seqno: -1,
							},
							"mariadb-galera-1": {
								UUID:  "00000000-0000-0000-0000-000000000000",
								Seqno: -1,
							},
							"mariadb-galera-2": {
								UUID:  "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
								Seqno: 1,
							},
						},
					},
				},
			},
			true,
		),
		Entry("partial state",
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Replicas: 3,
				},
				Status: mariadbv1alpha1.MariaDBStatus{
					GaleraRecovery: &mariadbv1alpha1.GaleraRecoveryStatus{
						State: map[string]*recovery.GaleraState{
							"mariadb-galera-0": {
								Version:         "2.1",
								UUID:            "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
								Seqno:           1,
								SafeToBootstrap: false,
							},
							"mariadb-galera-1": {
								Version:         "2.1",
								UUID:            "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
								Seqno:           1,
								SafeToBootstrap: false,
							},
						},
					},
				},
			},
			false,
		),
		Entry("full state",
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Replicas: 3,
				},
				Status: mariadbv1alpha1.MariaDBStatus{
					GaleraRecovery: &mariadbv1alpha1.GaleraRecoveryStatus{
						State: map[string]*recovery.GaleraState{
							"mariadb-galera-0": {
								Version:         "2.1",
								UUID:            "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
								Seqno:           1,
								SafeToBootstrap: false,
							},
							"mariadb-galera-1": {
								Version:         "2.1",
								UUID:            "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
								Seqno:           1,
								SafeToBootstrap: false,
							},
							"mariadb-galera-2": {
								Version:         "2.1",
								UUID:            "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
								Seqno:           1,
								SafeToBootstrap: false,
							},
						},
					},
				},
			},
			true,
		),
		Entry("partially recovered",
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Replicas: 3,
				},
				Status: mariadbv1alpha1.MariaDBStatus{
					GaleraRecovery: &mariadbv1alpha1.GaleraRecoveryStatus{
						Recovered: map[string]*recovery.Bootstrap{
							"mariadb-galera-0": {
								UUID:  "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
								Seqno: 1,
							},
							"mariadb-galera-2": {
								UUID:  "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
								Seqno: 1,
							},
						},
					},
				},
			},
			false,
		),
		Entry("fully recovered",
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Replicas: 3,
				},
				Status: mariadbv1alpha1.MariaDBStatus{
					GaleraRecovery: &mariadbv1alpha1.GaleraRecoveryStatus{
						Recovered: map[string]*recovery.Bootstrap{
							"mariadb-galera-0": {
								UUID:  "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
								Seqno: 1,
							},
							"mariadb-galera-1": {
								UUID:  "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
								Seqno: 1,
							},
							"mariadb-galera-2": {
								UUID:  "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
								Seqno: 1,
							},
						},
					},
				},
			},
			true,
		),
		Entry("incomplete",
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Replicas: 3,
				},
				Status: mariadbv1alpha1.MariaDBStatus{
					GaleraRecovery: &mariadbv1alpha1.GaleraRecoveryStatus{
						State: map[string]*recovery.GaleraState{
							"mariadb-galera-0": {
								Version:         "2.1",
								UUID:            "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
								Seqno:           1,
								SafeToBootstrap: false,
							},
							"mariadb-galera-1": {
								Version:         "2.1",
								UUID:            "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
								Seqno:           1,
								SafeToBootstrap: false,
							},
						},
						Recovered: map[string]*recovery.Bootstrap{
							"mariadb-galera-0": {
								UUID:  "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
								Seqno: 1,
							},
						},
					},
				},
			},
			false,
		),
		Entry("incomplete seqnos",
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Replicas: 3,
				},
				Status: mariadbv1alpha1.MariaDBStatus{
					GaleraRecovery: &mariadbv1alpha1.GaleraRecoveryStatus{
						State: map[string]*recovery.GaleraState{
							"mariadb-galera-0": {
								Version:         "2.1",
								UUID:            "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
								Seqno:           1,
								SafeToBootstrap: false,
							},
							"mariadb-galera-1": {
								Version:         "2.1",
								UUID:            "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
								Seqno:           -1,
								SafeToBootstrap: false,
							},
						},
						Recovered: map[string]*recovery.Bootstrap{
							"mariadb-galera-2": {
								UUID:  "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
								Seqno: 1,
							},
						},
					},
				},
			},
			false,
		),
		Entry("complete",
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Replicas: 3,
				},
				Status: mariadbv1alpha1.MariaDBStatus{
					GaleraRecovery: &mariadbv1alpha1.GaleraRecoveryStatus{
						State: map[string]*recovery.GaleraState{
							"mariadb-galera-0": {
								Version:         "2.1",
								UUID:            "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
								Seqno:           1,
								SafeToBootstrap: false,
							},
							"mariadb-galera-1": {
								Version:         "2.1",
								UUID:            "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
								Seqno:           1,
								SafeToBootstrap: false,
							},
						},
						Recovered: map[string]*recovery.Bootstrap{
							"mariadb-galera-2": {
								UUID:  "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
								Seqno: 1,
							},
						},
					},
				},
			},
			true,
		),
		Entry("complete with intersection",
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Replicas: 3,
				},
				Status: mariadbv1alpha1.MariaDBStatus{
					GaleraRecovery: &mariadbv1alpha1.GaleraRecoveryStatus{
						State: map[string]*recovery.GaleraState{
							"mariadb-galera-0": {
								Version:         "2.1",
								UUID:            "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
								Seqno:           1,
								SafeToBootstrap: false,
							},
							"mariadb-galera-1": {
								Version:         "2.1",
								UUID:            "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
								Seqno:           1,
								SafeToBootstrap: false,
							},
						},
						Recovered: map[string]*recovery.Bootstrap{
							"mariadb-galera-1": {
								UUID:  "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
								Seqno: 1,
							},
							"mariadb-galera-2": {
								UUID:  "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
								Seqno: 1,
							},
						},
					},
				},
			},
			true,
		),
		Entry("fully complete",
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Replicas: 3,
				},
				Status: mariadbv1alpha1.MariaDBStatus{
					GaleraRecovery: &mariadbv1alpha1.GaleraRecoveryStatus{
						State: map[string]*recovery.GaleraState{
							"mariadb-galera-0": {
								Version:         "2.1",
								UUID:            "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
								Seqno:           -1,
								SafeToBootstrap: false,
							},
							"mariadb-galera-1": {
								Version:         "2.1",
								UUID:            "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
								Seqno:           1,
								SafeToBootstrap: false,
							},
							"mariadb-galera-2": {
								Version:         "2.1",
								UUID:            "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
								Seqno:           1,
								SafeToBootstrap: false,
							},
						},
						Recovered: map[string]*recovery.Bootstrap{
							"mariadb-galera-0": {
								UUID:  "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
								Seqno: 1,
							},
							"mariadb-galera-1": {
								UUID:  "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
								Seqno: 1,
							},
							"mariadb-galera-2": {
								UUID:  "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
								Seqno: 1,
							},
						},
					},
				},
			},
			true,
		),
		Entry("some skipped Pods",
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Replicas: 3,
				},
				Status: mariadbv1alpha1.MariaDBStatus{
					GaleraRecovery: &mariadbv1alpha1.GaleraRecoveryStatus{
						State: map[string]*recovery.GaleraState{
							"mariadb-galera-0": {
								Version:         "2.1",
								UUID:            "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
								Seqno:           1,
								SafeToBootstrap: false,
							},
							"mariadb-galera-1": {
								Version:         "2.1",
								UUID:            "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
								Seqno:           1,
								SafeToBootstrap: false,
							},
							"mariadb-galera-2": {
								Version:         "2.1",
								UUID:            "00000000-0000-0000-0000-000000000000",
								Seqno:           -1,
								SafeToBootstrap: false,
							},
						},
						Recovered: map[string]*recovery.Bootstrap{
							"mariadb-galera-0": {
								UUID:  "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
								Seqno: 1,
							},
							"mariadb-galera-1": {
								UUID:  "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
								Seqno: 1,
							},
							"mariadb-galera-2": {
								UUID:  "00000000-0000-0000-0000-000000000000",
								Seqno: -1,
							},
						},
					},
				},
			},
			true,
		),
		Entry("all skipped Pods",
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Replicas: 3,
				},
				Status: mariadbv1alpha1.MariaDBStatus{
					GaleraRecovery: &mariadbv1alpha1.GaleraRecoveryStatus{
						State: map[string]*recovery.GaleraState{
							"mariadb-galera-0": {
								Version:         "2.1",
								UUID:            "00000000-0000-0000-0000-000000000000",
								Seqno:           -1,
								SafeToBootstrap: false,
							},
							"mariadb-galera-1": {
								Version:         "2.1",
								UUID:            "00000000-0000-0000-0000-000000000000",
								Seqno:           -1,
								SafeToBootstrap: false,
							},
							"mariadb-galera-2": {
								Version:         "2.1",
								UUID:            "00000000-0000-0000-0000-000000000000",
								Seqno:           -1,
								SafeToBootstrap: false,
							},
						},
						Recovered: map[string]*recovery.Bootstrap{
							"mariadb-galera-0": {
								UUID:  "00000000-0000-0000-0000-000000000000",
								Seqno: -1,
							},
							"mariadb-galera-1": {
								UUID:  "00000000-0000-0000-0000-000000000000",
								Seqno: -1,
							},
							"mariadb-galera-2": {
								UUID:  "00000000-0000-0000-0000-000000000000",
								Seqno: -1,
							},
						},
					},
				},
			},
			false,
		),
		Entry("recover from all zero UUIDs",
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Replicas: 3,
				},
				Status: mariadbv1alpha1.MariaDBStatus{
					GaleraRecovery: &mariadbv1alpha1.GaleraRecoveryStatus{
						State: map[string]*recovery.GaleraState{
							"mariadb-galera-0": {
								Version:         "2.1",
								UUID:            "00000000-0000-0000-0000-000000000000",
								Seqno:           -1,
								SafeToBootstrap: false,
							},
							"mariadb-galera-1": {
								Version:         "2.1",
								UUID:            "00000000-0000-0000-0000-000000000000",
								Seqno:           -1,
								SafeToBootstrap: false,
							},
							"mariadb-galera-2": {
								Version:         "2.1",
								UUID:            "00000000-0000-0000-0000-000000000000",
								Seqno:           -1,
								SafeToBootstrap: false,
							},
						},
						Recovered: map[string]*recovery.Bootstrap{
							"mariadb-galera-0": {
								UUID:  "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
								Seqno: 1,
							},
							"mariadb-galera-1": {
								UUID:  "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
								Seqno: 1,
							},
							"mariadb-galera-2": {
								UUID:  "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
								Seqno: 1,
							},
						},
					},
				},
			},
			true,
		),
		Entry("recover all zero UUIDs",
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Replicas: 3,
				},
				Status: mariadbv1alpha1.MariaDBStatus{
					GaleraRecovery: &mariadbv1alpha1.GaleraRecoveryStatus{
						State: map[string]*recovery.GaleraState{
							"mariadb-galera-0": {
								Version:         "2.1",
								UUID:            "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
								Seqno:           -1,
								SafeToBootstrap: false,
							},
							"mariadb-galera-1": {
								Version:         "2.1",
								UUID:            "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
								Seqno:           -1,
								SafeToBootstrap: false,
							},
							"mariadb-galera-2": {
								Version:         "2.1",
								UUID:            "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
								Seqno:           -1,
								SafeToBootstrap: false,
							},
						},
						Recovered: map[string]*recovery.Bootstrap{
							"mariadb-galera-0": {
								UUID:  "00000000-0000-0000-0000-000000000000",
								Seqno: 1,
							},
							"mariadb-galera-1": {
								UUID:  "00000000-0000-0000-0000-000000000000",
								Seqno: 1,
							},
							"mariadb-galera-2": {
								UUID:  "00000000-0000-0000-0000-000000000000",
								Seqno: 1,
							},
						},
					},
				},
			},
			true,
		),
		Entry("recover all zero UUIDs and some zero seqno",
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Replicas: 3,
				},
				Status: mariadbv1alpha1.MariaDBStatus{
					GaleraRecovery: &mariadbv1alpha1.GaleraRecoveryStatus{
						State: map[string]*recovery.GaleraState{
							"mariadb-galera-0": {
								Version:         "2.1",
								UUID:            "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
								Seqno:           -1,
								SafeToBootstrap: false,
							},
							"mariadb-galera-1": {
								Version:         "2.1",
								UUID:            "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
								Seqno:           -1,
								SafeToBootstrap: false,
							},
							"mariadb-galera-2": {
								Version:         "2.1",
								UUID:            "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
								Seqno:           -1,
								SafeToBootstrap: false,
							},
						},
						Recovered: map[string]*recovery.Bootstrap{
							"mariadb-galera-0": {
								UUID:  "00000000-0000-0000-0000-000000000000",
								Seqno: 0,
							},
							"mariadb-galera-1": {
								UUID:  "00000000-0000-0000-0000-000000000000",
								Seqno: 1,
							},
							"mariadb-galera-2": {
								UUID:  "00000000-0000-0000-0000-000000000000",
								Seqno: 1,
							},
						},
					},
				},
			},
			true,
		),
	)
})

var _ = Describe("RecoveryStatus bootstrapSource", func() {
	objMeta := metav1.ObjectMeta{
		Name: "mariadb-galera",
	}
	pod0 := "mariadb-galera-0"
	pod1 := "mariadb-galera-1"
	pod2 := "mariadb-galera-2"

	DescribeTable("returns the bootstrap source",
		func(mdb *mariadbv1alpha1.MariaDB, forceBootstrapInPod *string, wantSource *bootstrapSource, wantErr bool) {
			rs := newRecoveryStatus(mdb)
			source, err := rs.bootstrapSource(mdb, forceBootstrapInPod, logr.Logger{})
			Expect(source).To(Equal(wantSource))
			if wantErr {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
			}
		},
		Entry("no status",
			&mariadbv1alpha1.MariaDB{},
			nil,
			nil,
			true,
		),
		Entry("missing pods",
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Replicas: 3,
				},
				Status: mariadbv1alpha1.MariaDBStatus{
					GaleraRecovery: &mariadbv1alpha1.GaleraRecoveryStatus{
						State: map[string]*recovery.GaleraState{
							"mariadb-galera-0": {
								Version:         "2.1",
								UUID:            "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
								Seqno:           1,
								SafeToBootstrap: false,
							},
							"mariadb-galera-1": {
								Version:         "2.1",
								UUID:            "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
								Seqno:           1,
								SafeToBootstrap: false,
							},
						},
						Recovered: map[string]*recovery.Bootstrap{
							"mariadb-galera-0": {
								UUID:  "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
								Seqno: 1,
							},
							"mariadb-galera-1": {
								UUID:  "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
								Seqno: 1,
							},
						},
					},
				},
			},
			nil,
			nil,
			true,
		),
		Entry("force bootstrap",
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Replicas: 3,
				},
				Status: mariadbv1alpha1.MariaDBStatus{
					GaleraRecovery: &mariadbv1alpha1.GaleraRecoveryStatus{
						State: map[string]*recovery.GaleraState{
							"mariadb-galera-0": {
								Version:         "2.1",
								UUID:            "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
								Seqno:           1,
								SafeToBootstrap: true,
							},
							"mariadb-galera-1": {
								Version:         "2.1",
								UUID:            "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
								Seqno:           1,
								SafeToBootstrap: false,
							},
						},
						Recovered: map[string]*recovery.Bootstrap{
							"mariadb-galera-1": {
								UUID:  "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
								Seqno: 1,
							},
							"mariadb-galera-2": {
								UUID:  "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
								Seqno: 1,
							},
						},
					},
				},
			},
			ptr.To("mariadb-galera-0"),
			&bootstrapSource{
				pod: pod0,
			},
			false,
		),
		Entry("force bootstrap in non existing Pod",
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Replicas: 3,
				},
				Status: mariadbv1alpha1.MariaDBStatus{
					GaleraRecovery: &mariadbv1alpha1.GaleraRecoveryStatus{
						State: map[string]*recovery.GaleraState{
							"mariadb-galera-0": {
								Version:         "2.1",
								UUID:            "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
								Seqno:           1,
								SafeToBootstrap: true,
							},
							"mariadb-galera-1": {
								Version:         "2.1",
								UUID:            "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
								Seqno:           1,
								SafeToBootstrap: false,
							},
						},
						Recovered: map[string]*recovery.Bootstrap{
							"mariadb-galera-1": {
								UUID:  "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
								Seqno: 1,
							},
							"mariadb-galera-2": {
								UUID:  "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
								Seqno: 1,
							},
						},
					},
				},
			},
			ptr.To("mariadb-galera-5"),
			nil,
			true,
		),
		Entry("safe to bootstrap",
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Replicas: 3,
				},
				Status: mariadbv1alpha1.MariaDBStatus{
					GaleraRecovery: &mariadbv1alpha1.GaleraRecoveryStatus{
						State: map[string]*recovery.GaleraState{
							"mariadb-galera-0": {
								Version:         "2.1",
								UUID:            "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
								Seqno:           1,
								SafeToBootstrap: false,
							},
							"mariadb-galera-1": {
								Version:         "2.1",
								UUID:            "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
								Seqno:           1,
								SafeToBootstrap: true,
							},
						},
						Recovered: map[string]*recovery.Bootstrap{
							"mariadb-galera-0": {
								UUID:  "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
								Seqno: 1,
							},
						},
					},
				},
			},
			nil,
			&bootstrapSource{
				bootstrap: &recovery.Bootstrap{
					UUID:  "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
					Seqno: 1,
				},
				pod: pod1,
			},
			false,
		),
		Entry("incomplete recovery",
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Replicas: 3,
				},
				Status: mariadbv1alpha1.MariaDBStatus{
					GaleraRecovery: &mariadbv1alpha1.GaleraRecoveryStatus{
						State: map[string]*recovery.GaleraState{
							"mariadb-galera-0": {
								Version:         "2.1",
								UUID:            "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
								Seqno:           1,
								SafeToBootstrap: false,
							},
							"mariadb-galera-1": {
								Version:         "2.1",
								UUID:            "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
								Seqno:           1,
								SafeToBootstrap: false,
							},
						},
						Recovered: map[string]*recovery.Bootstrap{
							"mariadb-galera-0": {
								UUID:  "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
								Seqno: 1,
							},
						},
					},
				},
			},
			nil,
			nil,
			true,
		),
		Entry("partially recovered",
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Replicas: 3,
				},
				Status: mariadbv1alpha1.MariaDBStatus{
					GaleraRecovery: &mariadbv1alpha1.GaleraRecoveryStatus{
						State: map[string]*recovery.GaleraState{
							"mariadb-galera-0": {
								Version:         "2.1",
								UUID:            "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
								Seqno:           1,
								SafeToBootstrap: false,
							},
							"mariadb-galera-1": {
								Version:         "2.1",
								UUID:            "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
								Seqno:           1,
								SafeToBootstrap: false,
							},
						},
						Recovered: map[string]*recovery.Bootstrap{
							"mariadb-galera-2": {
								UUID:  "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
								Seqno: 1,
							},
						},
					},
				},
			},
			nil,
			&bootstrapSource{
				bootstrap: &recovery.Bootstrap{
					UUID:  "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
					Seqno: 1,
				},
				pod: pod2,
			},
			false,
		),
		Entry("partially recovered with different seqnos",
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Replicas: 3,
				},
				Status: mariadbv1alpha1.MariaDBStatus{
					GaleraRecovery: &mariadbv1alpha1.GaleraRecoveryStatus{
						State: map[string]*recovery.GaleraState{
							"mariadb-galera-0": {
								Version:         "2.1",
								UUID:            "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
								Seqno:           4,
								SafeToBootstrap: false,
							},
							"mariadb-galera-1": {
								Version:         "2.1",
								UUID:            "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
								Seqno:           8,
								SafeToBootstrap: false,
							},
						},
						Recovered: map[string]*recovery.Bootstrap{
							"mariadb-galera-2": {
								UUID:  "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
								Seqno: 6,
							},
						},
					},
				},
			},
			nil,
			&bootstrapSource{
				bootstrap: &recovery.Bootstrap{
					UUID:  "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
					Seqno: 8,
				},
				pod: pod1,
			},
			false,
		),
		Entry("partially recovered with different seqnos and safe to bootstrap",
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Replicas: 3,
				},
				Status: mariadbv1alpha1.MariaDBStatus{
					GaleraRecovery: &mariadbv1alpha1.GaleraRecoveryStatus{
						State: map[string]*recovery.GaleraState{
							"mariadb-galera-0": {
								Version:         "2.1",
								UUID:            "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
								Seqno:           4,
								SafeToBootstrap: true,
							},
							"mariadb-galera-1": {
								Version:         "2.1",
								UUID:            "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
								Seqno:           8,
								SafeToBootstrap: false,
							},
						},
						Recovered: map[string]*recovery.Bootstrap{
							"mariadb-galera-2": {
								UUID:  "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
								Seqno: 6,
							},
						},
					},
				},
			},
			nil,
			&bootstrapSource{
				bootstrap: &recovery.Bootstrap{
					UUID:  "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
					Seqno: 4,
				},
				pod: pod0,
			},
			false,
		),
		Entry("partially recovered with skipped Pods",
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Replicas: 3,
				},
				Status: mariadbv1alpha1.MariaDBStatus{
					GaleraRecovery: &mariadbv1alpha1.GaleraRecoveryStatus{
						State: map[string]*recovery.GaleraState{
							"mariadb-galera-0": {
								Version:         "2.1",
								UUID:            "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
								Seqno:           4,
								SafeToBootstrap: true,
							},
							"mariadb-galera-1": {
								Version:         "2.1",
								UUID:            "00000000-0000-0000-0000-000000000000",
								Seqno:           -1,
								SafeToBootstrap: false,
							},
							"mariadb-galera-2": {
								Version:         "2.1",
								UUID:            "00000000-0000-0000-0000-000000000000",
								Seqno:           -1,
								SafeToBootstrap: false,
							},
						},
						Recovered: map[string]*recovery.Bootstrap{
							"mariadb-galera-1": {
								UUID:  "00000000-0000-0000-0000-000000000000",
								Seqno: -1,
							},
							"mariadb-galera-2": {
								UUID:  "00000000-0000-0000-0000-000000000000",
								Seqno: -1,
							},
						},
					},
				},
			},
			nil,
			&bootstrapSource{
				bootstrap: &recovery.Bootstrap{
					UUID:  "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
					Seqno: 4,
				},
				pod: pod0,
			},
			false,
		),
		Entry("fully recovered",
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Replicas: 3,
				},
				Status: mariadbv1alpha1.MariaDBStatus{
					GaleraRecovery: &mariadbv1alpha1.GaleraRecoveryStatus{
						State: map[string]*recovery.GaleraState{
							"mariadb-galera-0": {
								Version:         "2.1",
								UUID:            "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
								Seqno:           1,
								SafeToBootstrap: false,
							},
							"mariadb-galera-1": {
								Version:         "2.1",
								UUID:            "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
								Seqno:           1,
								SafeToBootstrap: false,
							},
							"mariadb-galera-2": {
								Version:         "2.1",
								UUID:            "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
								Seqno:           1,
								SafeToBootstrap: false,
							},
						},
						Recovered: map[string]*recovery.Bootstrap{
							"mariadb-galera-0": {
								UUID:  "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
								Seqno: 1,
							},
							"mariadb-galera-1": {
								UUID:  "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
								Seqno: 1,
							},
							"mariadb-galera-2": {
								UUID:  "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
								Seqno: 1,
							},
						},
					},
				},
			},
			nil,
			&bootstrapSource{
				bootstrap: &recovery.Bootstrap{
					UUID:  "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
					Seqno: 1,
				},
				pod: pod2,
			},
			false,
		),
		Entry("fully recovered with different seqnos",
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Replicas: 3,
				},
				Status: mariadbv1alpha1.MariaDBStatus{
					GaleraRecovery: &mariadbv1alpha1.GaleraRecoveryStatus{
						State: map[string]*recovery.GaleraState{
							"mariadb-galera-0": {
								Version:         "2.1",
								UUID:            "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
								Seqno:           3,
								SafeToBootstrap: false,
							},
							"mariadb-galera-1": {
								Version:         "2.1",
								UUID:            "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
								Seqno:           6,
								SafeToBootstrap: false,
							},
							"mariadb-galera-2": {
								Version:         "2.1",
								UUID:            "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
								Seqno:           1,
								SafeToBootstrap: false,
							},
						},
						Recovered: map[string]*recovery.Bootstrap{
							"mariadb-galera-0": {
								UUID:  "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
								Seqno: 2,
							},
							"mariadb-galera-1": {
								UUID:  "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
								Seqno: 3,
							},
							"mariadb-galera-2": {
								UUID:  "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
								Seqno: 9,
							},
						},
					},
				},
			},
			nil,
			&bootstrapSource{
				bootstrap: &recovery.Bootstrap{
					UUID:  "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
					Seqno: 9,
				},
				pod: pod2,
			},
			false,
		),
		Entry("fully recovered with different seqnos and safe to bootstrap",
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Replicas: 3,
				},
				Status: mariadbv1alpha1.MariaDBStatus{
					GaleraRecovery: &mariadbv1alpha1.GaleraRecoveryStatus{
						State: map[string]*recovery.GaleraState{
							"mariadb-galera-0": {
								Version:         "2.1",
								UUID:            "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
								Seqno:           3,
								SafeToBootstrap: false,
							},
							"mariadb-galera-1": {
								Version:         "2.1",
								UUID:            "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
								Seqno:           6,
								SafeToBootstrap: true,
							},
							"mariadb-galera-2": {
								Version:         "2.1",
								UUID:            "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
								Seqno:           1,
								SafeToBootstrap: false,
							},
						},
						Recovered: map[string]*recovery.Bootstrap{
							"mariadb-galera-0": {
								UUID:  "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
								Seqno: 2,
							},
							"mariadb-galera-1": {
								UUID:  "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
								Seqno: 3,
							},
							"mariadb-galera-2": {
								UUID:  "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
								Seqno: 9,
							},
						},
					},
				},
			},
			nil,
			&bootstrapSource{
				bootstrap: &recovery.Bootstrap{
					UUID:  "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
					Seqno: 6,
				},
				pod: pod1,
			},
			false,
		),
		Entry("fully recovered with skipped Pods",
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Replicas: 3,
				},
				Status: mariadbv1alpha1.MariaDBStatus{
					GaleraRecovery: &mariadbv1alpha1.GaleraRecoveryStatus{
						State: map[string]*recovery.GaleraState{
							"mariadb-galera-0": {
								Version:         "2.1",
								UUID:            "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
								Seqno:           3,
								SafeToBootstrap: false,
							},
							"mariadb-galera-1": {
								Version:         "2.1",
								UUID:            "00000000-0000-0000-0000-000000000000",
								Seqno:           -1,
								SafeToBootstrap: false,
							},
							"mariadb-galera-2": {
								Version:         "2.1",
								UUID:            "00000000-0000-0000-0000-000000000000",
								Seqno:           -1,
								SafeToBootstrap: false,
							},
						},
						Recovered: map[string]*recovery.Bootstrap{
							"mariadb-galera-0": {
								UUID:  "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
								Seqno: 3,
							},
							"mariadb-galera-1": {
								UUID:  "00000000-0000-0000-0000-000000000000",
								Seqno: -1,
							},
							"mariadb-galera-2": {
								UUID:  "00000000-0000-0000-0000-000000000000",
								Seqno: -1,
							},
						},
					},
				},
			},
			nil,
			&bootstrapSource{
				bootstrap: &recovery.Bootstrap{
					UUID:  "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
					Seqno: 3,
				},
				pod: pod0,
			},
			false,
		),
		Entry("fully recovered with zero UUIDs",
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Replicas: 3,
				},
				Status: mariadbv1alpha1.MariaDBStatus{
					GaleraRecovery: &mariadbv1alpha1.GaleraRecoveryStatus{
						State: map[string]*recovery.GaleraState{
							"mariadb-galera-0": {
								Version:         "2.1",
								UUID:            "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
								Seqno:           -1,
								SafeToBootstrap: false,
							},
							"mariadb-galera-1": {
								Version:         "2.1",
								UUID:            "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
								Seqno:           -1,
								SafeToBootstrap: false,
							},
							"mariadb-galera-2": {
								Version:         "2.1",
								UUID:            "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
								Seqno:           -1,
								SafeToBootstrap: false,
							},
						},
						Recovered: map[string]*recovery.Bootstrap{
							"mariadb-galera-0": {
								UUID:  "00000000-0000-0000-0000-000000000000",
								Seqno: 1,
							},
							"mariadb-galera-1": {
								UUID:  "00000000-0000-0000-0000-000000000000",
								Seqno: 1,
							},
							"mariadb-galera-2": {
								UUID:  "00000000-0000-0000-0000-000000000000",
								Seqno: 1,
							},
						},
					},
				},
			},
			nil,
			&bootstrapSource{
				bootstrap: &recovery.Bootstrap{
					UUID:  "00000000-0000-0000-0000-000000000000",
					Seqno: 1,
				},
				pod: pod2,
			},
			false,
		),
		Entry("fully recovered with zero UUIDs and some zero seqnos",
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Replicas: 3,
				},
				Status: mariadbv1alpha1.MariaDBStatus{
					GaleraRecovery: &mariadbv1alpha1.GaleraRecoveryStatus{
						State: map[string]*recovery.GaleraState{
							"mariadb-galera-0": {
								Version:         "2.1",
								UUID:            "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
								Seqno:           -1,
								SafeToBootstrap: false,
							},
							"mariadb-galera-1": {
								Version:         "2.1",
								UUID:            "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
								Seqno:           -1,
								SafeToBootstrap: false,
							},
							"mariadb-galera-2": {
								Version:         "2.1",
								UUID:            "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
								Seqno:           -1,
								SafeToBootstrap: false,
							},
						},
						Recovered: map[string]*recovery.Bootstrap{
							"mariadb-galera-0": {
								UUID:  "00000000-0000-0000-0000-000000000000",
								Seqno: 0,
							},
							"mariadb-galera-1": {
								UUID:  "00000000-0000-0000-0000-000000000000",
								Seqno: 1,
							},
							"mariadb-galera-2": {
								UUID:  "00000000-0000-0000-0000-000000000000",
								Seqno: 1,
							},
						},
					},
				},
			},
			nil,
			&bootstrapSource{
				bootstrap: &recovery.Bootstrap{
					UUID:  "00000000-0000-0000-0000-000000000000",
					Seqno: 1,
				},
				pod: pod2,
			},
			false,
		),
	)
})

var _ = Describe("RecoveryStatus podsRestarted", func() {
	It("reports whether Pods have been restarted", func() {
		timeout := 3 * time.Second
		duration := metav1.Duration{Duration: timeout}
		mdb := &mariadbv1alpha1.MariaDB{
			Spec: mariadbv1alpha1.MariaDBSpec{
				Galera: &mariadbv1alpha1.Galera{
					Enabled: true,
					GaleraSpec: mariadbv1alpha1.GaleraSpec{
						Recovery: &mariadbv1alpha1.GaleraRecovery{
							Enabled:                 true,
							ClusterHealthyTimeout:   &duration,
							ClusterBootstrapTimeout: &duration,
							PodRecoveryTimeout:      &duration,
							PodSyncTimeout:          &duration,
						},
					},
				},
			},
		}
		rs := newRecoveryStatus(mdb)
		Expect(rs.podsRestarted()).To(BeFalse())

		rs.setPodsRestarted(true)
		Expect(rs.podsRestarted()).To(BeTrue())
	})
})
