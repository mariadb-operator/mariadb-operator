package galera

import (
	"reflect"
	"testing"
	"time"

	"github.com/go-logr/logr"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/pkg/galera/recovery"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func TestRecoveryStatusGetSet(t *testing.T) {
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
	if !ok {
		t.Error("expect mariadb-galera-1 state to be found")
	}
	if !reflect.DeepEqual(state1, gotState) {
		t.Errorf("unexpected state value: expected: %v, got: %v", state1, gotState)
	}
	gotState, ok = rs.state("foo-0")
	if ok {
		t.Error("expect state not to be found")
	}
	if gotState != nil {
		t.Errorf("unexpected state value: expected: %v, got: %v", nil, gotState)
	}

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
	if !ok {
		t.Error("expect mariadb-galera-0 state to be found")
	}
	if !reflect.DeepEqual(recovered0, gotRecovered) {
		t.Errorf("unexpected recovered value: expected: %v, got: %v", recovered0, gotRecovered)
	}
	gotRecovered, ok = rs.recovered("foo-1")
	if ok {
		t.Error("expect recovered not to be found")
	}
	if gotState != nil {
		t.Errorf("unexpected recovered value: expected: %v, got: %v", nil, gotRecovered)
	}

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
	if !reflect.DeepEqual(expectedRecoveryStatus, gotRecoveryStatus) {
		t.Errorf("unexpected recovery status value: expected: %v, got: %v", expectedRecoveryStatus, gotRecoveryStatus)
	}

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
	if !reflect.DeepEqual(expectedRecoveryStatus, gotRecoveryStatus) {
		t.Errorf("unexpected recovery status value: expected: %v, got: %v", expectedRecoveryStatus, gotRecoveryStatus)
	}
}

func TestRecoveryStatusIsComplete(t *testing.T) {
	objMeta := metav1.ObjectMeta{
		Name: "mariadb-galera",
	}
	tests := []struct {
		name     string
		mdb      *mariadbv1alpha1.MariaDB
		wantBool bool
	}{
		// {
		// 	name:     "no status",
		// 	mdb:      &mariadbv1alpha1.MariaDB{},
		// 	wantBool: false,
		// },
		{
			name: "missing pods",
			mdb: &mariadbv1alpha1.MariaDB{
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
			wantBool: false,
		},
		{
			name: "safe to bootstrap",
			mdb: &mariadbv1alpha1.MariaDB{
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
			wantBool: true,
		},
		{
			name: "safe to bootstrap with negative seqnos",
			mdb: &mariadbv1alpha1.MariaDB{
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
			wantBool: true,
		},
		{
			name: "safe to bootstrap with skipped Pods",
			mdb: &mariadbv1alpha1.MariaDB{
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
			wantBool: true,
		},
		{
			name: "partial state",
			mdb: &mariadbv1alpha1.MariaDB{
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
			wantBool: false,
		},
		{
			name: "full state",
			mdb: &mariadbv1alpha1.MariaDB{
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
			wantBool: true,
		},
		{
			name: "partially recovered",
			mdb: &mariadbv1alpha1.MariaDB{
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
			wantBool: false,
		},
		{
			name: "fully recovered",
			mdb: &mariadbv1alpha1.MariaDB{
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
			wantBool: true,
		},
		{
			name: "incomplete",
			mdb: &mariadbv1alpha1.MariaDB{
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
			wantBool: false,
		},
		{
			name: "incomplete seqnos",
			mdb: &mariadbv1alpha1.MariaDB{
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
			wantBool: false,
		},
		{
			name: "complete",
			mdb: &mariadbv1alpha1.MariaDB{
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
			wantBool: true,
		},
		{
			name: "complete with intersection",
			mdb: &mariadbv1alpha1.MariaDB{
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
			wantBool: true,
		},
		{
			name: "fully complete",
			mdb: &mariadbv1alpha1.MariaDB{
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
			wantBool: true,
		},
		{
			name: "some skipped Pods",
			mdb: &mariadbv1alpha1.MariaDB{
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
			wantBool: true,
		},
		{
			name: "all skipped Pods",
			mdb: &mariadbv1alpha1.MariaDB{
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
			wantBool: false,
		},
		{
			name: "recover from all zero UUIDs",
			mdb: &mariadbv1alpha1.MariaDB{
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
			wantBool: true,
		},
		{
			name: "recover all zero UUIDs",
			mdb: &mariadbv1alpha1.MariaDB{
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
			wantBool: true,
		},
		{
			name: "recover all zero UUIDs and some zero seqno",
			mdb: &mariadbv1alpha1.MariaDB{
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
			wantBool: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rs := newRecoveryStatus(tt.mdb)
			complete := rs.isComplete(tt.mdb, logr.Logger{})
			if tt.wantBool != complete {
				t.Errorf("unexpected complete value: expected: %v, got: %v", tt.wantBool, complete)
			}
		})
	}
}

func TestRecoveryStatusBootstrapSource(t *testing.T) {
	objMeta := metav1.ObjectMeta{
		Name: "mariadb-galera",
	}
	pod0 := "mariadb-galera-0"
	pod1 := "mariadb-galera-1"
	pod2 := "mariadb-galera-2"
	tests := []struct {
		name                string
		mdb                 *mariadbv1alpha1.MariaDB
		pods                []corev1.Pod
		forceBootstrapInPod *string
		wantSource          *bootstrapSource
		wantErr             bool
	}{
		{
			name:       "no status",
			mdb:        &mariadbv1alpha1.MariaDB{},
			wantSource: nil,
			wantErr:    true,
		},
		{
			name: "missing pods",
			mdb: &mariadbv1alpha1.MariaDB{
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
			forceBootstrapInPod: nil,
			wantSource:          nil,
			wantErr:             true,
		},
		{
			name: "force bootstrap",
			mdb: &mariadbv1alpha1.MariaDB{
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
			forceBootstrapInPod: ptr.To("mariadb-galera-0"),
			wantSource: &bootstrapSource{
				pod: pod0,
			},
			wantErr: false,
		},
		{
			name: "force bootstrap in non existing Pod",
			mdb: &mariadbv1alpha1.MariaDB{
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
			forceBootstrapInPod: ptr.To("mariadb-galera-5"),
			wantSource:          nil,
			wantErr:             true,
		},
		{
			name: "safe to bootstrap",
			mdb: &mariadbv1alpha1.MariaDB{
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
			forceBootstrapInPod: nil,
			wantSource: &bootstrapSource{
				bootstrap: &recovery.Bootstrap{
					UUID:  "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
					Seqno: 1,
				},
				pod: pod1,
			},
			wantErr: false,
		},
		{
			name: "incomplete recovery",
			mdb: &mariadbv1alpha1.MariaDB{
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
			forceBootstrapInPod: nil,
			wantSource:          nil,
			wantErr:             true,
		},
		{
			name: "partially recovered",
			mdb: &mariadbv1alpha1.MariaDB{
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
			forceBootstrapInPod: nil,
			wantSource: &bootstrapSource{
				bootstrap: &recovery.Bootstrap{
					UUID:  "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
					Seqno: 1,
				},
				pod: pod2,
			},
			wantErr: false,
		},
		{
			name: "partially recovered with different seqnos",
			mdb: &mariadbv1alpha1.MariaDB{
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
			forceBootstrapInPod: nil,
			wantSource: &bootstrapSource{
				bootstrap: &recovery.Bootstrap{
					UUID:  "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
					Seqno: 8,
				},
				pod: pod1,
			},
			wantErr: false,
		},
		{
			name: "partially recovered with different seqnos and safe to bootstrap",
			mdb: &mariadbv1alpha1.MariaDB{
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
			forceBootstrapInPod: nil,
			wantSource: &bootstrapSource{
				bootstrap: &recovery.Bootstrap{
					UUID:  "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
					Seqno: 4,
				},
				pod: pod0,
			},
			wantErr: false,
		},
		{
			name: "partially recovered with skipped Pods",
			mdb: &mariadbv1alpha1.MariaDB{
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
			forceBootstrapInPod: nil,
			wantSource: &bootstrapSource{
				bootstrap: &recovery.Bootstrap{
					UUID:  "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
					Seqno: 4,
				},
				pod: pod0,
			},
			wantErr: false,
		},
		{
			name: "fully recovered",
			mdb: &mariadbv1alpha1.MariaDB{
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
			forceBootstrapInPod: nil,
			wantSource: &bootstrapSource{
				bootstrap: &recovery.Bootstrap{
					UUID:  "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
					Seqno: 1,
				},
				pod: pod2,
			},
			wantErr: false,
		},
		{
			name: "fully recovered with different seqnos",
			mdb: &mariadbv1alpha1.MariaDB{
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
			forceBootstrapInPod: nil,
			wantSource: &bootstrapSource{
				bootstrap: &recovery.Bootstrap{
					UUID:  "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
					Seqno: 9,
				},
				pod: pod2,
			},
			wantErr: false,
		},
		{
			name: "fully recovered with different seqnos and safe to bootstrap",
			mdb: &mariadbv1alpha1.MariaDB{
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
			forceBootstrapInPod: nil,
			wantSource: &bootstrapSource{
				bootstrap: &recovery.Bootstrap{
					UUID:  "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
					Seqno: 6,
				},
				pod: pod1,
			},
			wantErr: false,
		},
		{
			name: "fully recovered with skipped Pods",
			mdb: &mariadbv1alpha1.MariaDB{
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
			forceBootstrapInPod: nil,
			wantSource: &bootstrapSource{
				bootstrap: &recovery.Bootstrap{
					UUID:  "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
					Seqno: 3,
				},
				pod: pod0,
			},
			wantErr: false,
		},
		{
			name: "fully recovered with zero UUIDs",
			mdb: &mariadbv1alpha1.MariaDB{
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
			forceBootstrapInPod: nil,
			wantSource: &bootstrapSource{
				bootstrap: &recovery.Bootstrap{
					UUID:  "00000000-0000-0000-0000-000000000000",
					Seqno: 1,
				},
				pod: pod2,
			},
			wantErr: false,
		},
		{
			name: "fully recovered with zero UUIDs and some zero seqnos",
			mdb: &mariadbv1alpha1.MariaDB{
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
			forceBootstrapInPod: nil,
			wantSource: &bootstrapSource{
				bootstrap: &recovery.Bootstrap{
					UUID:  "00000000-0000-0000-0000-000000000000",
					Seqno: 1,
				},
				pod: pod2,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rs := newRecoveryStatus(tt.mdb)
			source, err := rs.bootstrapSource(tt.mdb, tt.forceBootstrapInPod, logr.Logger{})
			if !reflect.DeepEqual(tt.wantSource, source) {
				t.Errorf("unexpected bootstrapSource value: expected: %v, got: %v", tt.wantSource, source)
			}
			if tt.wantErr && err == nil {
				t.Error("expect error to have occurred, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("expect error to not have occurred, got: %v", err)
			}
		})
	}
}

func TestRecoveryStatusPodsRestarted(t *testing.T) {
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
	if rs.podsRestarted() {
		t.Error("expect recovery status to not have Pods restarted")
	}

	rs.setPodsRestarted(true)
	if !rs.podsRestarted() {
		t.Error("expect recovery status to have Pods restarted")
	}
}
