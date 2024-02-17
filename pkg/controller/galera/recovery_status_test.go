package galera

import (
	"reflect"
	"testing"
	"time"

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
		UUID:            "dfc4e849-1c90-43b0-a2c8-0b777c1ce6e4",
		Seqno:           3,
		SafeToBootstrap: false,
	}
	state1 := &recovery.GaleraState{
		Version:         "2.1",
		UUID:            "0fc0436e-560f-4951-ae97-16911aae7ecf",
		Seqno:           6,
		SafeToBootstrap: true,
	}
	state2 := &recovery.GaleraState{
		Version:         "2.1",
		UUID:            "1ef327e6-8579-4d8e-bd3c-6f3f99e40b1d",
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
		UUID:  "dfc4e849-1c90-43b0-a2c8-0b777c1ce6e4",
		Seqno: 2,
	}
	recovered1 := &recovery.Bootstrap{
		UUID:  "0fc0436e-560f-4951-ae97-16911aae7ecf",
		Seqno: 3,
	}
	recovered2 := &recovery.Bootstrap{
		UUID:  "1ef327e6-8579-4d8e-bd3c-6f3f99e40b1d",
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

func TestRecoveryStatusBootstrap(t *testing.T) {
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
	if rs.isBootstrapping() {
		t.Error("expect recovery status to not allow bootstrapping")
	}
	if rs.bootstrapTimeout(&mariadbv1alpha1.MariaDB{}) {
		t.Error("expect recovery status bootstrap not to have timed out")
	}

	rs.setBootstrapping("mariadb-galera-0")
	if !rs.isBootstrapping() {
		t.Error("expect recovery status to allow bootstrapping")
	}
	if rs.bootstrapTimeout(mdb) {
		t.Error("expect recovery status bootstrap not to have timed out")
	}

	time.Sleep(timeout)
	if !rs.bootstrapTimeout(mdb) {
		t.Error("expect recovery status bootstrap to have timed out")
	}
}

func TestRecoveryStatusSafeToBootstrap(t *testing.T) {
	pod0 := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "mariadb-galera-0",
		},
	}
	pod1 := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "mariadb-galera-1",
		},
	}
	pod2 := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "mariadb-galera-2",
		},
	}
	pods := []corev1.Pod{pod0, pod1, pod2}
	tests := []struct {
		name       string
		mdb        *mariadbv1alpha1.MariaDB
		pods       []corev1.Pod
		wantSource *bootstrapSource
		wantErr    bool
	}{
		{
			name:       "no status",
			mdb:        &mariadbv1alpha1.MariaDB{},
			pods:       pods,
			wantSource: nil,
			wantErr:    true,
		},
		{
			name: "no pods",
			mdb: &mariadbv1alpha1.MariaDB{
				Status: mariadbv1alpha1.MariaDBStatus{
					GaleraRecovery: &mariadbv1alpha1.GaleraRecoveryStatus{
						State: map[string]*recovery.GaleraState{
							"mariadb-galera-0": {
								Version:         "2.1",
								UUID:            "dfc4e849-1c90-43b0-a2c8-0b777c1ce6e4",
								Seqno:           -1,
								SafeToBootstrap: false,
							},
							"mariadb-galera-1": {
								Version:         "2.1",
								UUID:            "0fc0436e-560f-4951-ae97-16911aae7ecf",
								Seqno:           1,
								SafeToBootstrap: true,
							},
							"mariadb-galera-2": {
								Version:         "2.1",
								UUID:            "1ef327e6-8579-4d8e-bd3c-6f3f99e40b1d",
								Seqno:           2,
								SafeToBootstrap: true,
							},
						},
					},
				},
			},
			pods:       []corev1.Pod{},
			wantSource: nil,
			wantErr:    true,
		},
		{
			name: "negative seqno",
			mdb: &mariadbv1alpha1.MariaDB{
				Status: mariadbv1alpha1.MariaDBStatus{
					GaleraRecovery: &mariadbv1alpha1.GaleraRecoveryStatus{
						State: map[string]*recovery.GaleraState{
							"mariadb-galera-0": {
								Version:         "2.1",
								UUID:            "dfc4e849-1c90-43b0-a2c8-0b777c1ce6e4",
								Seqno:           -1,
								SafeToBootstrap: false,
							},
							"mariadb-galera-1": {
								Version:         "2.1",
								UUID:            "0fc0436e-560f-4951-ae97-16911aae7ecf",
								Seqno:           -1,
								SafeToBootstrap: true,
							},
							"mariadb-galera-2": {
								Version:         "2.1",
								UUID:            "1ef327e6-8579-4d8e-bd3c-6f3f99e40b1d",
								Seqno:           -1,
								SafeToBootstrap: true,
							},
						},
					},
				},
			},
			pods:       pods,
			wantSource: nil,
			wantErr:    true,
		},
		{
			name: "no source",
			mdb: &mariadbv1alpha1.MariaDB{
				Status: mariadbv1alpha1.MariaDBStatus{
					GaleraRecovery: &mariadbv1alpha1.GaleraRecoveryStatus{
						State: map[string]*recovery.GaleraState{
							"mariadb-galera-0": {
								Version:         "2.1",
								UUID:            "dfc4e849-1c90-43b0-a2c8-0b777c1ce6e4",
								Seqno:           -1,
								SafeToBootstrap: false,
							},
							"mariadb-galera-1": {
								Version:         "2.1",
								UUID:            "0fc0436e-560f-4951-ae97-16911aae7ecf",
								Seqno:           1,
								SafeToBootstrap: false,
							},
							"mariadb-galera-2": {
								Version:         "2.1",
								UUID:            "1ef327e6-8579-4d8e-bd3c-6f3f99e40b1d",
								Seqno:           2,
								SafeToBootstrap: false,
							},
						},
					},
				},
			},
			pods:       pods,
			wantSource: nil,
			wantErr:    true,
		},
		{
			name: "safe to bootstrap source",
			mdb: &mariadbv1alpha1.MariaDB{
				Status: mariadbv1alpha1.MariaDBStatus{
					GaleraRecovery: &mariadbv1alpha1.GaleraRecoveryStatus{
						State: map[string]*recovery.GaleraState{
							"mariadb-galera-0": {
								Version:         "2.1",
								UUID:            "dfc4e849-1c90-43b0-a2c8-0b777c1ce6e4",
								Seqno:           -1,
								SafeToBootstrap: false,
							},
							"mariadb-galera-1": {
								Version:         "2.1",
								UUID:            "0fc0436e-560f-4951-ae97-16911aae7ecf",
								Seqno:           1,
								SafeToBootstrap: true,
							},
							"mariadb-galera-2": {
								Version:         "2.1",
								UUID:            "1ef327e6-8579-4d8e-bd3c-6f3f99e40b1d",
								Seqno:           2,
								SafeToBootstrap: false,
							},
						},
					},
				},
			},
			pods: pods,
			wantSource: &bootstrapSource{
				bootstrap: &recovery.Bootstrap{
					UUID:  "0fc0436e-560f-4951-ae97-16911aae7ecf",
					Seqno: 1,
				},
				pod: &pod1,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rs := newRecoveryStatus(tt.mdb)
			source, err := rs.safeToBootstrap(tt.pods)
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

func TestRecoveryStatusIsComplete(t *testing.T) {
	pods := []corev1.Pod{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "mariadb-galera-0",
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "mariadb-galera-1",
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "mariadb-galera-2",
			},
		},
	}
	tests := []struct {
		name     string
		mdb      *mariadbv1alpha1.MariaDB
		pods     []corev1.Pod
		wantBool bool
	}{
		{
			name:     "no status",
			mdb:      &mariadbv1alpha1.MariaDB{},
			pods:     pods,
			wantBool: false,
		},
		{
			name:     "no pods",
			mdb:      &mariadbv1alpha1.MariaDB{},
			pods:     []corev1.Pod{},
			wantBool: false,
		},
		{
			name: "partial state",
			mdb: &mariadbv1alpha1.MariaDB{
				Status: mariadbv1alpha1.MariaDBStatus{
					GaleraRecovery: &mariadbv1alpha1.GaleraRecoveryStatus{
						State: map[string]*recovery.GaleraState{
							"mariadb-galera-0": {
								Version:         "2.1",
								UUID:            "dfc4e849-1c90-43b0-a2c8-0b777c1ce6e4",
								Seqno:           1,
								SafeToBootstrap: false,
							},
							"mariadb-galera-1": {
								Version:         "2.1",
								UUID:            "0fc0436e-560f-4951-ae97-16911aae7ecf",
								Seqno:           1,
								SafeToBootstrap: true,
							},
						},
					},
				},
			},
			pods:     pods,
			wantBool: false,
		},
		{
			name: "full state",
			mdb: &mariadbv1alpha1.MariaDB{
				Status: mariadbv1alpha1.MariaDBStatus{
					GaleraRecovery: &mariadbv1alpha1.GaleraRecoveryStatus{
						State: map[string]*recovery.GaleraState{
							"mariadb-galera-0": {
								Version:         "2.1",
								UUID:            "dfc4e849-1c90-43b0-a2c8-0b777c1ce6e4",
								Seqno:           1,
								SafeToBootstrap: false,
							},
							"mariadb-galera-1": {
								Version:         "2.1",
								UUID:            "0fc0436e-560f-4951-ae97-16911aae7ecf",
								Seqno:           1,
								SafeToBootstrap: false,
							},
							"mariadb-galera-2": {
								Version:         "2.1",
								UUID:            "1ef327e6-8579-4d8e-bd3c-6f3f99e40b1d",
								Seqno:           1,
								SafeToBootstrap: true,
							},
						},
					},
				},
			},
			pods:     pods,
			wantBool: true,
		},
		{
			name: "partially recovered",
			mdb: &mariadbv1alpha1.MariaDB{
				Status: mariadbv1alpha1.MariaDBStatus{
					GaleraRecovery: &mariadbv1alpha1.GaleraRecoveryStatus{
						Recovered: map[string]*recovery.Bootstrap{
							"mariadb-galera-0": {
								UUID:  "dfc4e849-1c90-43b0-a2c8-0b777c1ce6e4",
								Seqno: 1,
							},
							"mariadb-galera-2": {
								UUID:  "1ef327e6-8579-4d8e-bd3c-6f3f99e40b1d",
								Seqno: 1,
							},
						},
					},
				},
			},
			pods:     pods,
			wantBool: false,
		},
		{
			name: "fully recovered",
			mdb: &mariadbv1alpha1.MariaDB{
				Status: mariadbv1alpha1.MariaDBStatus{
					GaleraRecovery: &mariadbv1alpha1.GaleraRecoveryStatus{
						Recovered: map[string]*recovery.Bootstrap{
							"mariadb-galera-0": {
								UUID:  "dfc4e849-1c90-43b0-a2c8-0b777c1ce6e4",
								Seqno: 1,
							},
							"mariadb-galera-1": {
								UUID:  "0fc0436e-560f-4951-ae97-16911aae7ecf",
								Seqno: 1,
							},
							"mariadb-galera-2": {
								UUID:  "1ef327e6-8579-4d8e-bd3c-6f3f99e40b1d",
								Seqno: 1,
							},
						},
					},
				},
			},
			pods:     pods,
			wantBool: true,
		},
		{
			name: "incomplete",
			mdb: &mariadbv1alpha1.MariaDB{
				Status: mariadbv1alpha1.MariaDBStatus{
					GaleraRecovery: &mariadbv1alpha1.GaleraRecoveryStatus{
						State: map[string]*recovery.GaleraState{
							"mariadb-galera-0": {
								Version:         "2.1",
								UUID:            "dfc4e849-1c90-43b0-a2c8-0b777c1ce6e4",
								Seqno:           1,
								SafeToBootstrap: true,
							},
							"mariadb-galera-1": {
								Version:         "2.1",
								UUID:            "0fc0436e-560f-4951-ae97-16911aae7ecf",
								Seqno:           1,
								SafeToBootstrap: true,
							},
						},
						Recovered: map[string]*recovery.Bootstrap{
							"mariadb-galera-0": {
								UUID:  "dfc4e849-1c90-43b0-a2c8-0b777c1ce6e4",
								Seqno: 1,
							},
						},
					},
				},
			},
			pods:     pods,
			wantBool: false,
		},
		{
			name: "incomplete seqnos",
			mdb: &mariadbv1alpha1.MariaDB{
				Status: mariadbv1alpha1.MariaDBStatus{
					GaleraRecovery: &mariadbv1alpha1.GaleraRecoveryStatus{
						State: map[string]*recovery.GaleraState{
							"mariadb-galera-0": {
								Version:         "2.1",
								UUID:            "dfc4e849-1c90-43b0-a2c8-0b777c1ce6e4",
								Seqno:           1,
								SafeToBootstrap: true,
							},
							"mariadb-galera-1": {
								Version:         "2.1",
								UUID:            "0fc0436e-560f-4951-ae97-16911aae7ecf",
								Seqno:           -1,
								SafeToBootstrap: true,
							},
						},
						Recovered: map[string]*recovery.Bootstrap{
							"mariadb-galera-2": {
								UUID:  "1ef327e6-8579-4d8e-bd3c-6f3f99e40b1d",
								Seqno: 1,
							},
						},
					},
				},
			},
			pods:     pods,
			wantBool: false,
		},
		{
			name: "complete",
			mdb: &mariadbv1alpha1.MariaDB{
				Status: mariadbv1alpha1.MariaDBStatus{
					GaleraRecovery: &mariadbv1alpha1.GaleraRecoveryStatus{
						State: map[string]*recovery.GaleraState{
							"mariadb-galera-0": {
								Version:         "2.1",
								UUID:            "dfc4e849-1c90-43b0-a2c8-0b777c1ce6e4",
								Seqno:           1,
								SafeToBootstrap: true,
							},
							"mariadb-galera-1": {
								Version:         "2.1",
								UUID:            "0fc0436e-560f-4951-ae97-16911aae7ecf",
								Seqno:           1,
								SafeToBootstrap: true,
							},
						},
						Recovered: map[string]*recovery.Bootstrap{
							"mariadb-galera-2": {
								UUID:  "1ef327e6-8579-4d8e-bd3c-6f3f99e40b1d",
								Seqno: 1,
							},
						},
					},
				},
			},
			pods:     pods,
			wantBool: true,
		},
		{
			name: "complete with intersection",
			mdb: &mariadbv1alpha1.MariaDB{
				Status: mariadbv1alpha1.MariaDBStatus{
					GaleraRecovery: &mariadbv1alpha1.GaleraRecoveryStatus{
						State: map[string]*recovery.GaleraState{
							"mariadb-galera-0": {
								Version:         "2.1",
								UUID:            "dfc4e849-1c90-43b0-a2c8-0b777c1ce6e4",
								Seqno:           1,
								SafeToBootstrap: true,
							},
							"mariadb-galera-1": {
								Version:         "2.1",
								UUID:            "0fc0436e-560f-4951-ae97-16911aae7ecf",
								Seqno:           1,
								SafeToBootstrap: true,
							},
						},
						Recovered: map[string]*recovery.Bootstrap{
							"mariadb-galera-1": {
								UUID:  "0fc0436e-560f-4951-ae97-16911aae7ecf",
								Seqno: 1,
							},
							"mariadb-galera-2": {
								UUID:  "1ef327e6-8579-4d8e-bd3c-6f3f99e40b1d",
								Seqno: 1,
							},
						},
					},
				},
			},
			pods:     pods,
			wantBool: true,
		},
		{
			name: "fully complete",
			mdb: &mariadbv1alpha1.MariaDB{
				Status: mariadbv1alpha1.MariaDBStatus{
					GaleraRecovery: &mariadbv1alpha1.GaleraRecoveryStatus{
						State: map[string]*recovery.GaleraState{
							"mariadb-galera-0": {
								Version:         "2.1",
								UUID:            "dfc4e849-1c90-43b0-a2c8-0b777c1ce6e4",
								Seqno:           -1,
								SafeToBootstrap: false,
							},
							"mariadb-galera-1": {
								Version:         "2.1",
								UUID:            "0fc0436e-560f-4951-ae97-16911aae7ecf",
								Seqno:           1,
								SafeToBootstrap: true,
							},
							"mariadb-galera-2": {
								Version:         "2.1",
								UUID:            "1ef327e6-8579-4d8e-bd3c-6f3f99e40b1d",
								Seqno:           1,
								SafeToBootstrap: true,
							},
						},
						Recovered: map[string]*recovery.Bootstrap{
							"mariadb-galera-0": {
								UUID:  "dfc4e849-1c90-43b0-a2c8-0b777c1ce6e4",
								Seqno: 1,
							},
							"mariadb-galera-1": {
								UUID:  "0fc0436e-560f-4951-ae97-16911aae7ecf",
								Seqno: 1,
							},
							"mariadb-galera-2": {
								UUID:  "1ef327e6-8579-4d8e-bd3c-6f3f99e40b1d",
								Seqno: 1,
							},
						},
					},
				},
			},
			pods:     pods,
			wantBool: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rs := newRecoveryStatus(tt.mdb)
			complete := rs.isComplete(tt.pods)
			if tt.wantBool != complete {
				t.Errorf("unexpected complete value: expected: %v, got: %v", tt.wantBool, complete)
			}
		})
	}
}

func TestRecoveryStatusBootstrapSource(t *testing.T) {
	pod0 := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "mariadb-galera-0",
		},
	}
	pod1 := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "mariadb-galera-1",
		},
	}
	pod2 := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "mariadb-galera-2",
		},
	}
	pods := []corev1.Pod{pod0, pod1, pod2}
	tests := []struct {
		name       string
		mdb        *mariadbv1alpha1.MariaDB
		pods       []corev1.Pod
		wantSource *bootstrapSource
		wantErr    bool
	}{
		{
			name:       "no status",
			mdb:        &mariadbv1alpha1.MariaDB{},
			pods:       pods,
			wantSource: nil,
			wantErr:    true,
		},
		{
			name: "missing pods",
			mdb: &mariadbv1alpha1.MariaDB{
				Status: mariadbv1alpha1.MariaDBStatus{
					GaleraRecovery: &mariadbv1alpha1.GaleraRecoveryStatus{
						State: map[string]*recovery.GaleraState{
							"mariadb-galera-0": {
								Version:         "2.1",
								UUID:            "dfc4e849-1c90-43b0-a2c8-0b777c1ce6e4",
								Seqno:           1,
								SafeToBootstrap: true,
							},
							"mariadb-galera-1": {
								Version:         "2.1",
								UUID:            "0fc0436e-560f-4951-ae97-16911aae7ecf",
								Seqno:           1,
								SafeToBootstrap: false,
							},
						},
						Recovered: map[string]*recovery.Bootstrap{
							"mariadb-galera-1": {
								UUID:  "0fc0436e-560f-4951-ae97-16911aae7ecf",
								Seqno: 1,
							},
							"mariadb-galera-2": {
								UUID:  "1ef327e6-8579-4d8e-bd3c-6f3f99e40b1d",
								Seqno: 1,
							},
						},
					},
				},
			},
			pods:       []corev1.Pod{},
			wantSource: nil,
			wantErr:    true,
		},
		{
			name: "safe to bootstrap",
			mdb: &mariadbv1alpha1.MariaDB{
				Status: mariadbv1alpha1.MariaDBStatus{
					GaleraRecovery: &mariadbv1alpha1.GaleraRecoveryStatus{
						State: map[string]*recovery.GaleraState{
							"mariadb-galera-0": {
								Version:         "2.1",
								UUID:            "dfc4e849-1c90-43b0-a2c8-0b777c1ce6e4",
								Seqno:           1,
								SafeToBootstrap: false,
							},
							"mariadb-galera-1": {
								Version:         "2.1",
								UUID:            "0fc0436e-560f-4951-ae97-16911aae7ecf",
								Seqno:           1,
								SafeToBootstrap: true,
							},
						},
						Recovered: map[string]*recovery.Bootstrap{
							"mariadb-galera-0": {
								UUID:  "dfc4e849-1c90-43b0-a2c8-0b777c1ce6e4",
								Seqno: 1,
							},
						},
					},
				},
			},
			pods: pods,
			wantSource: &bootstrapSource{
				bootstrap: &recovery.Bootstrap{
					UUID:  "0fc0436e-560f-4951-ae97-16911aae7ecf",
					Seqno: 1,
				},
				pod: &pod1,
			},
			wantErr: false,
		},
		{
			name: "incomplete recovery",
			mdb: &mariadbv1alpha1.MariaDB{
				Status: mariadbv1alpha1.MariaDBStatus{
					GaleraRecovery: &mariadbv1alpha1.GaleraRecoveryStatus{
						State: map[string]*recovery.GaleraState{
							"mariadb-galera-0": {
								Version:         "2.1",
								UUID:            "dfc4e849-1c90-43b0-a2c8-0b777c1ce6e4",
								Seqno:           1,
								SafeToBootstrap: false,
							},
							"mariadb-galera-1": {
								Version:         "2.1",
								UUID:            "0fc0436e-560f-4951-ae97-16911aae7ecf",
								Seqno:           1,
								SafeToBootstrap: false,
							},
						},
						Recovered: map[string]*recovery.Bootstrap{
							"mariadb-galera-0": {
								UUID:  "dfc4e849-1c90-43b0-a2c8-0b777c1ce6e4",
								Seqno: 1,
							},
						},
					},
				},
			},
			pods:       pods,
			wantSource: nil,
			wantErr:    true,
		},
		{
			name: "partially recovered",
			mdb: &mariadbv1alpha1.MariaDB{
				Status: mariadbv1alpha1.MariaDBStatus{
					GaleraRecovery: &mariadbv1alpha1.GaleraRecoveryStatus{
						State: map[string]*recovery.GaleraState{
							"mariadb-galera-0": {
								Version:         "2.1",
								UUID:            "dfc4e849-1c90-43b0-a2c8-0b777c1ce6e4",
								Seqno:           1,
								SafeToBootstrap: false,
							},
							"mariadb-galera-1": {
								Version:         "2.1",
								UUID:            "0fc0436e-560f-4951-ae97-16911aae7ecf",
								Seqno:           1,
								SafeToBootstrap: false,
							},
						},
						Recovered: map[string]*recovery.Bootstrap{
							"mariadb-galera-2": {
								UUID:  "1ef327e6-8579-4d8e-bd3c-6f3f99e40b1d",
								Seqno: 1,
							},
						},
					},
				},
			},
			pods: pods,
			wantSource: &bootstrapSource{
				bootstrap: &recovery.Bootstrap{
					UUID:  "1ef327e6-8579-4d8e-bd3c-6f3f99e40b1d",
					Seqno: 1,
				},
				pod: &pod2,
			},
			wantErr: false,
		},
		{
			name: "partially recovered with different seqnos",
			mdb: &mariadbv1alpha1.MariaDB{
				Status: mariadbv1alpha1.MariaDBStatus{
					GaleraRecovery: &mariadbv1alpha1.GaleraRecoveryStatus{
						State: map[string]*recovery.GaleraState{
							"mariadb-galera-0": {
								Version:         "2.1",
								UUID:            "dfc4e849-1c90-43b0-a2c8-0b777c1ce6e4",
								Seqno:           4,
								SafeToBootstrap: false,
							},
							"mariadb-galera-1": {
								Version:         "2.1",
								UUID:            "0fc0436e-560f-4951-ae97-16911aae7ecf",
								Seqno:           8,
								SafeToBootstrap: false,
							},
						},
						Recovered: map[string]*recovery.Bootstrap{
							"mariadb-galera-2": {
								UUID:  "1ef327e6-8579-4d8e-bd3c-6f3f99e40b1d",
								Seqno: 6,
							},
						},
					},
				},
			},
			pods: pods,
			wantSource: &bootstrapSource{
				bootstrap: &recovery.Bootstrap{
					UUID:  "0fc0436e-560f-4951-ae97-16911aae7ecf",
					Seqno: 8,
				},
				pod: &pod1,
			},
			wantErr: false,
		},
		{
			name: "partially recovered with different seqnos and safe to bootstrap",
			mdb: &mariadbv1alpha1.MariaDB{
				Status: mariadbv1alpha1.MariaDBStatus{
					GaleraRecovery: &mariadbv1alpha1.GaleraRecoveryStatus{
						State: map[string]*recovery.GaleraState{
							"mariadb-galera-0": {
								Version:         "2.1",
								UUID:            "dfc4e849-1c90-43b0-a2c8-0b777c1ce6e4",
								Seqno:           4,
								SafeToBootstrap: true,
							},
							"mariadb-galera-1": {
								Version:         "2.1",
								UUID:            "0fc0436e-560f-4951-ae97-16911aae7ecf",
								Seqno:           8,
								SafeToBootstrap: false,
							},
						},
						Recovered: map[string]*recovery.Bootstrap{
							"mariadb-galera-2": {
								UUID:  "1ef327e6-8579-4d8e-bd3c-6f3f99e40b1d",
								Seqno: 6,
							},
						},
					},
				},
			},
			pods: pods,
			wantSource: &bootstrapSource{
				bootstrap: &recovery.Bootstrap{
					UUID:  "dfc4e849-1c90-43b0-a2c8-0b777c1ce6e4",
					Seqno: 4,
				},
				pod: &pod0,
			},
			wantErr: false,
		},
		{
			name: "fully recovered",
			mdb: &mariadbv1alpha1.MariaDB{
				Status: mariadbv1alpha1.MariaDBStatus{
					GaleraRecovery: &mariadbv1alpha1.GaleraRecoveryStatus{
						State: map[string]*recovery.GaleraState{
							"mariadb-galera-0": {
								Version:         "2.1",
								UUID:            "dfc4e849-1c90-43b0-a2c8-0b777c1ce6e4",
								Seqno:           1,
								SafeToBootstrap: false,
							},
							"mariadb-galera-1": {
								Version:         "2.1",
								UUID:            "0fc0436e-560f-4951-ae97-16911aae7ecf",
								Seqno:           1,
								SafeToBootstrap: false,
							},
							"mariadb-galera-2": {
								Version:         "2.1",
								UUID:            "1ef327e6-8579-4d8e-bd3c-6f3f99e40b1d",
								Seqno:           1,
								SafeToBootstrap: false,
							},
						},
						Recovered: map[string]*recovery.Bootstrap{
							"mariadb-galera-0": {
								UUID:  "dfc4e849-1c90-43b0-a2c8-0b777c1ce6e4",
								Seqno: 1,
							},
							"mariadb-galera-1": {
								UUID:  "0fc0436e-560f-4951-ae97-16911aae7ecf",
								Seqno: 1,
							},
							"mariadb-galera-2": {
								UUID:  "1ef327e6-8579-4d8e-bd3c-6f3f99e40b1d",
								Seqno: 1,
							},
						},
					},
				},
			},
			pods: pods,
			wantSource: &bootstrapSource{
				bootstrap: &recovery.Bootstrap{
					UUID:  "1ef327e6-8579-4d8e-bd3c-6f3f99e40b1d",
					Seqno: 1,
				},
				pod: &pod2,
			},
			wantErr: false,
		},
		{
			name: "fully recovered with different seqnos",
			mdb: &mariadbv1alpha1.MariaDB{
				Status: mariadbv1alpha1.MariaDBStatus{
					GaleraRecovery: &mariadbv1alpha1.GaleraRecoveryStatus{
						State: map[string]*recovery.GaleraState{
							"mariadb-galera-0": {
								Version:         "2.1",
								UUID:            "dfc4e849-1c90-43b0-a2c8-0b777c1ce6e4",
								Seqno:           3,
								SafeToBootstrap: false,
							},
							"mariadb-galera-1": {
								Version:         "2.1",
								UUID:            "0fc0436e-560f-4951-ae97-16911aae7ecf",
								Seqno:           6,
								SafeToBootstrap: false,
							},
							"mariadb-galera-2": {
								Version:         "2.1",
								UUID:            "1ef327e6-8579-4d8e-bd3c-6f3f99e40b1d",
								Seqno:           1,
								SafeToBootstrap: false,
							},
						},
						Recovered: map[string]*recovery.Bootstrap{
							"mariadb-galera-0": {
								UUID:  "dfc4e849-1c90-43b0-a2c8-0b777c1ce6e4",
								Seqno: 2,
							},
							"mariadb-galera-1": {
								UUID:  "0fc0436e-560f-4951-ae97-16911aae7ecf",
								Seqno: 3,
							},
							"mariadb-galera-2": {
								UUID:  "1ef327e6-8579-4d8e-bd3c-6f3f99e40b1d",
								Seqno: 9,
							},
						},
					},
				},
			},
			pods: pods,
			wantSource: &bootstrapSource{
				bootstrap: &recovery.Bootstrap{
					UUID:  "1ef327e6-8579-4d8e-bd3c-6f3f99e40b1d",
					Seqno: 9,
				},
				pod: &pod2,
			},
			wantErr: false,
		},
		{
			name: "fully recovered with different seqnos and safe to bootstrap",
			mdb: &mariadbv1alpha1.MariaDB{
				Status: mariadbv1alpha1.MariaDBStatus{
					GaleraRecovery: &mariadbv1alpha1.GaleraRecoveryStatus{
						State: map[string]*recovery.GaleraState{
							"mariadb-galera-0": {
								Version:         "2.1",
								UUID:            "dfc4e849-1c90-43b0-a2c8-0b777c1ce6e4",
								Seqno:           3,
								SafeToBootstrap: false,
							},
							"mariadb-galera-1": {
								Version:         "2.1",
								UUID:            "0fc0436e-560f-4951-ae97-16911aae7ecf",
								Seqno:           6,
								SafeToBootstrap: true,
							},
							"mariadb-galera-2": {
								Version:         "2.1",
								UUID:            "1ef327e6-8579-4d8e-bd3c-6f3f99e40b1d",
								Seqno:           1,
								SafeToBootstrap: false,
							},
						},
						Recovered: map[string]*recovery.Bootstrap{
							"mariadb-galera-0": {
								UUID:  "dfc4e849-1c90-43b0-a2c8-0b777c1ce6e4",
								Seqno: 2,
							},
							"mariadb-galera-1": {
								UUID:  "0fc0436e-560f-4951-ae97-16911aae7ecf",
								Seqno: 3,
							},
							"mariadb-galera-2": {
								UUID:  "1ef327e6-8579-4d8e-bd3c-6f3f99e40b1d",
								Seqno: 9,
							},
						},
					},
				},
			},
			pods: pods,
			wantSource: &bootstrapSource{
				bootstrap: &recovery.Bootstrap{
					UUID:  "0fc0436e-560f-4951-ae97-16911aae7ecf",
					Seqno: 6,
				},
				pod: &pod1,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rs := newRecoveryStatus(tt.mdb)
			source, err := rs.bootstrapSource(tt.pods)
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
