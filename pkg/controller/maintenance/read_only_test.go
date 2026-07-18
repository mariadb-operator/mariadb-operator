package maintenance

import (
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"
)

var _ = Describe("GetReadOnlyDesiredPodState", func() {
	DescribeTable("computes the desired read-only pod state",
		func(mariadb *mariadbv1alpha1.MariaDB, expectedState map[int]bool) {
			r := &MaintenanceReconciler{}
			got := r.getReadOnlyDesiredPodState(mariadb)
			Expect(got).To(Equal(expectedState))
		},
		Entry("replication without maintenance",
			&mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Replicas: 3,
					Replication: &mariadbv1alpha1.Replication{
						Enabled: true,
					},
				},
				Status: mariadbv1alpha1.MariaDBStatus{
					CurrentPrimaryPodIndex: ptr.To(0),
				},
			},
			map[int]bool{
				0: false,
				1: true,
				2: true,
			},
		),
		Entry("replication without maintenance - primary at index 1",
			&mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Replicas: 3,
					Replication: &mariadbv1alpha1.Replication{
						Enabled: true,
					},
				},
				Status: mariadbv1alpha1.MariaDBStatus{
					CurrentPrimaryPodIndex: ptr.To(1),
				},
			},
			map[int]bool{
				0: true,
				1: false,
				2: true,
			},
		),
		Entry("replication with maintenance and readOnly",
			&mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Replicas: 3,
					Replication: &mariadbv1alpha1.Replication{
						Enabled: true,
					},
					Maintenance: ptr.To(mariadbv1alpha1.MariaDBMaintenance{
						Enabled:  true,
						ReadOnly: true,
					}),
				},
				Status: mariadbv1alpha1.MariaDBStatus{
					CurrentPrimaryPodIndex: ptr.To(0),
				},
			},
			map[int]bool{
				0: true,
				1: true,
				2: true,
			},
		),
		Entry("replication with maintenance and readOnly - primary at index 1",
			&mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Replicas: 3,
					Replication: &mariadbv1alpha1.Replication{
						Enabled: true,
					},
					Maintenance: ptr.To(mariadbv1alpha1.MariaDBMaintenance{
						Enabled:  true,
						ReadOnly: true,
					}),
				},
				Status: mariadbv1alpha1.MariaDBStatus{
					CurrentPrimaryPodIndex: ptr.To(1),
				},
			},
			map[int]bool{
				0: true,
				1: true,
				2: true,
			},
		),
		Entry("replication with maintenance without readOnly",
			&mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Replicas: 3,
					Replication: &mariadbv1alpha1.Replication{
						Enabled: true,
					},
					Maintenance: ptr.To(mariadbv1alpha1.MariaDBMaintenance{
						Enabled:  true,
						ReadOnly: false,
					}),
				},
				Status: mariadbv1alpha1.MariaDBStatus{
					CurrentPrimaryPodIndex: ptr.To(0),
				},
			},
			map[int]bool{
				0: false,
				1: true,
				2: true,
			},
		),
		Entry("replication with maintenance without readOnly - primary at index 1",
			&mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Replicas: 3,
					Replication: &mariadbv1alpha1.Replication{
						Enabled: true,
					},
					Maintenance: ptr.To(mariadbv1alpha1.MariaDBMaintenance{
						Enabled:  true,
						ReadOnly: false,
					}),
				},
				Status: mariadbv1alpha1.MariaDBStatus{
					CurrentPrimaryPodIndex: ptr.To(1),
				},
			},
			map[int]bool{
				0: true,
				1: false,
				2: true,
			},
		),
		Entry("standalone without maintenance",
			&mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Replicas: 1,
				},
			},
			map[int]bool{
				0: false,
			},
		),
		Entry("standalone with maintenance and readOnly",
			&mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Replicas: 1,
					Maintenance: ptr.To(mariadbv1alpha1.MariaDBMaintenance{
						Enabled:  true,
						ReadOnly: true,
					}),
				},
			},
			map[int]bool{
				0: true,
			},
		),
		Entry("standalone with maintenance without readOnly",
			&mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Replicas: 1,
					Maintenance: ptr.To(mariadbv1alpha1.MariaDBMaintenance{
						Enabled:  true,
						ReadOnly: false,
					}),
				},
			},
			map[int]bool{
				0: false,
			},
		),
		Entry("galera without maintenance",
			&mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Replicas: 3,
					Galera: ptr.To(mariadbv1alpha1.Galera{
						Enabled: true,
					}),
				},
				Status: mariadbv1alpha1.MariaDBStatus{
					CurrentPrimaryPodIndex: ptr.To(0),
				},
			},
			map[int]bool{
				0: false,
				1: false,
				2: false,
			},
		),
		Entry("galera without maintenance - primary at index 1",
			&mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Replicas: 3,
					Galera: ptr.To(mariadbv1alpha1.Galera{
						Enabled: true,
					}),
				},
				Status: mariadbv1alpha1.MariaDBStatus{
					CurrentPrimaryPodIndex: ptr.To(1),
				},
			},
			map[int]bool{
				0: false,
				1: false,
				2: false,
			},
		),
		Entry("galera with maintenance and readOnly",
			&mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Replicas: 3,
					Galera: ptr.To(mariadbv1alpha1.Galera{
						Enabled: true,
					}),
					Maintenance: ptr.To(mariadbv1alpha1.MariaDBMaintenance{
						Enabled:  true,
						ReadOnly: true,
					}),
				},
				Status: mariadbv1alpha1.MariaDBStatus{
					CurrentPrimaryPodIndex: ptr.To(0),
				},
			},
			map[int]bool{
				0: true,
				1: true,
				2: true,
			},
		),
		Entry("galera with maintenance and readOnly - primary at index 1",
			&mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Replicas: 3,
					Galera: ptr.To(mariadbv1alpha1.Galera{
						Enabled: true,
					}),
					Maintenance: ptr.To(mariadbv1alpha1.MariaDBMaintenance{
						Enabled:  true,
						ReadOnly: true,
					}),
				},
				Status: mariadbv1alpha1.MariaDBStatus{
					CurrentPrimaryPodIndex: ptr.To(1),
				},
			},
			map[int]bool{
				0: true,
				1: true,
				2: true,
			},
		),
		Entry("galera with maintenance without readOnly",
			&mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Replicas: 3,
					Galera: ptr.To(mariadbv1alpha1.Galera{
						Enabled: true,
					}),
					Maintenance: ptr.To(mariadbv1alpha1.MariaDBMaintenance{
						Enabled:  true,
						ReadOnly: false,
					}),
				},
				Status: mariadbv1alpha1.MariaDBStatus{
					CurrentPrimaryPodIndex: ptr.To(0),
				},
			},
			map[int]bool{
				0: false,
				1: false,
				2: false,
			},
		),
		Entry("galera with maintenance without readOnly - primary at index 1",
			&mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Replicas: 3,
					Galera: ptr.To(mariadbv1alpha1.Galera{
						Enabled: true,
					}),
					Maintenance: ptr.To(mariadbv1alpha1.MariaDBMaintenance{
						Enabled:  true,
						ReadOnly: false,
					}),
				},
				Status: mariadbv1alpha1.MariaDBStatus{
					CurrentPrimaryPodIndex: ptr.To(1),
				},
			},
			map[int]bool{
				0: false,
				1: false,
				2: false,
			},
		),
	)
})
