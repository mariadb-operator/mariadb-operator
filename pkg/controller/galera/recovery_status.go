package galera

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/go-logr/logr"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/pkg/datastructures"
	"github.com/mariadb-operator/mariadb-operator/pkg/galera/recovery"
	"github.com/mariadb-operator/mariadb-operator/pkg/statefulset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

type recoveryStatus struct {
	inner mariadbv1alpha1.GaleraRecoveryStatus
	mux   *sync.RWMutex
}

type bootstrapSource struct {
	bootstrap *recovery.Bootstrap
	pod       string
}

func (b *bootstrapSource) String() string {
	return fmt.Sprintf(
		"{ bootstrap: { UUID: %s, seqno: %d }, pod: %s }",
		b.bootstrap.UUID,
		b.bootstrap.Seqno,
		b.pod,
	)
}

func newRecoveryStatus(mariadb *mariadbv1alpha1.MariaDB) *recoveryStatus {
	inner := mariadbv1alpha1.GaleraRecoveryStatus{}
	galeraRecovery := ptr.Deref(mariadb.Status.GaleraRecovery, mariadbv1alpha1.GaleraRecoveryStatus{})

	if galeraRecovery.State != nil {
		inner.State = galeraRecovery.State
	}
	if galeraRecovery.Recovered != nil {
		inner.Recovered = galeraRecovery.Recovered
	}
	if galeraRecovery.Bootstrap != nil {
		inner.Bootstrap = galeraRecovery.Bootstrap
	}
	if galeraRecovery.PodsRestarted != nil {
		inner.PodsRestarted = galeraRecovery.PodsRestarted
	}

	return &recoveryStatus{
		inner: inner,
		mux:   &sync.RWMutex{},
	}
}
func (rs *recoveryStatus) galeraRecoveryStatus() mariadbv1alpha1.GaleraRecoveryStatus {
	rs.mux.RLock()
	defer rs.mux.RUnlock()

	return rs.inner
}

func (rs *recoveryStatus) setState(pod string, state *recovery.GaleraState) {
	rs.mux.Lock()
	defer rs.mux.Unlock()

	if rs.inner.State == nil {
		rs.inner.State = make(map[string]*recovery.GaleraState)
	}
	rs.inner.State[pod] = state
}

func (rs *recoveryStatus) state(pod string) (*recovery.GaleraState, bool) {
	rs.mux.RLock()
	defer rs.mux.RUnlock()

	state, ok := rs.inner.State[pod]
	return state, ok
}

func (rs *recoveryStatus) setRecovered(pod string, bootstrap *recovery.Bootstrap) {
	rs.mux.Lock()
	defer rs.mux.Unlock()

	if rs.inner.Recovered == nil {
		rs.inner.Recovered = make(map[string]*recovery.Bootstrap)
	}
	rs.inner.Recovered[pod] = bootstrap
}

func (rs *recoveryStatus) recovered(pod string) (*recovery.Bootstrap, bool) {
	rs.mux.RLock()
	defer rs.mux.RUnlock()

	bootstrap, ok := rs.inner.Recovered[pod]
	return bootstrap, ok
}

func (rs *recoveryStatus) reset() {
	rs.mux.Lock()
	defer rs.mux.Unlock()

	rs.inner = mariadbv1alpha1.GaleraRecoveryStatus{}
}

func (rs *recoveryStatus) setBootstrapping(pod string) {
	rs.mux.Lock()
	defer rs.mux.Unlock()

	rs.inner.Bootstrap = &mariadbv1alpha1.GaleraBootstrapStatus{
		Time: ptr.To(metav1.NewTime(time.Now())),
		Pod:  &pod,
	}
}

func (rs *recoveryStatus) isBootstrapping() bool {
	rs.mux.RLock()
	defer rs.mux.RUnlock()

	return rs.inner.Bootstrap != nil
}

func (rs *recoveryStatus) bootstrapTimeout(mdb *mariadbv1alpha1.MariaDB) bool {
	if !rs.isBootstrapping() {
		return false
	}
	rs.mux.RLock()
	defer rs.mux.RUnlock()

	if rs.inner.Bootstrap.Time == nil {
		return false
	}

	galera := ptr.Deref(mdb.Spec.Galera, mariadbv1alpha1.Galera{})
	recovery := ptr.Deref(galera.Recovery, mariadbv1alpha1.GaleraRecovery{})
	timeout := ptr.Deref(recovery.ClusterBootstrapTimeout, metav1.Duration{Duration: 10 * time.Minute}).Duration

	deadline := rs.inner.Bootstrap.Time.Add(timeout)
	return time.Now().After(deadline)
}

func (rs *recoveryStatus) isComplete(mdb *mariadbv1alpha1.MariaDB, logger logr.Logger) bool {
	rs.mux.RLock()
	defer rs.mux.RUnlock()

	pods := getPodNames(mdb)

	numSkippedPods := 0
	isComplete := true
	for _, p := range pods {
		state := ptr.Deref(rs.inner.State[p], recovery.GaleraState{})
		recovered := ptr.Deref(rs.inner.Recovered[p], recovery.Bootstrap{})

		if state.SafeToBootstrap {
			return true
		}
		if shouldSkipPod(recovered) {
			numSkippedPods++
			continue
		}
		if state.GetSeqno() > 0 || recovered.GetSeqno() > 0 {
			continue
		}
		isComplete = false
	}

	if numSkippedPods == len(pods) {
		logger.Info("Recovery status not completed: all Pods have been skipped")
		return false
	}
	return isComplete
}

func (rs *recoveryStatus) bootstrapSource(mdb *mariadbv1alpha1.MariaDB, forceBootstrapInPod *string,
	logger logr.Logger) (*bootstrapSource, error) {
	pods := getPodNames(mdb)

	if forceBootstrapInPod != nil {
		pod := datastructures.Find(pods, func(pod string) bool {
			return pod == *forceBootstrapInPod
		})
		if pod != nil {
			return &bootstrapSource{
				pod: *pod,
			}, nil
		}
		return nil, fmt.Errorf("Pod '%s' used to forcefully bootstrap not found", *forceBootstrapInPod)
	}

	if !rs.isComplete(mdb, logger) {
		return nil, errors.New("recovery status not completed")
	}

	rs.mux.RLock()
	defer rs.mux.RUnlock()
	var currentSource recovery.GaleraRecoverer
	var currentPod string

	for _, p := range pods {
		state := ptr.Deref(rs.inner.State[p], recovery.GaleraState{})
		recovered := ptr.Deref(rs.inner.Recovered[p], recovery.Bootstrap{})

		if state.SafeToBootstrap {
			return &bootstrapSource{
				bootstrap: &recovery.Bootstrap{
					UUID:  state.GetUUID(),
					Seqno: state.GetSeqno(),
				},
				pod: p,
			}, nil
		}
		if shouldSkipPod(recovered) {
			logger.Info("Skipping Pod while looking for a bootstrap source", "pod", p)
			continue
		}
		if state.GetSeqno() > 0 && state.Compare(currentSource) >= 0 {
			currentSource = &state
			currentPod = p
		}
		if recovered.GetSeqno() > 0 && recovered.Compare(currentSource) >= 0 {
			currentSource = &recovered
			currentPod = p
		}
	}

	if currentSource == nil {
		return nil, errors.New("bootstrap source not found")
	}
	return &bootstrapSource{
		bootstrap: &recovery.Bootstrap{
			UUID:  currentSource.GetUUID(),
			Seqno: currentSource.GetSeqno(),
		},
		pod: currentPod,
	}, nil
}

func (rs *recoveryStatus) setPodsRestarted(restarted bool) {
	rs.mux.Lock()
	defer rs.mux.Unlock()

	rs.inner.PodsRestarted = ptr.To(restarted)
}

func (rs *recoveryStatus) podsRestarted() bool {
	rs.mux.RLock()
	defer rs.mux.RUnlock()

	return ptr.Deref(rs.inner.PodsRestarted, false)
}

func getPodNames(mdb *mariadbv1alpha1.MariaDB) []string {
	podNames := make([]string, int(mdb.Spec.Replicas))
	for i := 0; i < int(mdb.Spec.Replicas); i++ {
		podNames[i] = statefulset.PodName(mdb.ObjectMeta, i)
	}
	return podNames
}

// shouldSkipPod determines whether a Pod should be skipped during the recovery process.
// UUID 00000000-0000-0000-0000-000000000000 means that the Pods needs SST to rejoin the cluster.
// See: https://galeracluster.com/library/documentation/node-provisioning.html#node-provisioning
// Seqno -1 does not really help determining the last running Pod.
func shouldSkipPod(recovered recovery.Bootstrap) bool {
	return recovered.UUID == "00000000-0000-0000-0000-000000000000" && recovered.Seqno == -1
}
