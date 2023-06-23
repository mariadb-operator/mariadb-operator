package galera

import (
	"errors"
	"sync"
	"time"

	agentgalera "github.com/mariadb-operator/agent/pkg/galera"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type recoveryStatus struct {
	inner *mariadbv1alpha1.GaleraRecoveryStatus
	mux   *sync.RWMutex
}

type bootstrapSource struct {
	bootstrap *agentgalera.Bootstrap
	pod       string
}

func newRecoveryStatus(mariadb *mariadbv1alpha1.MariaDB) *recoveryStatus {
	var inner mariadbv1alpha1.GaleraRecoveryStatus
	if mariadb.Status.GaleraRecovery != nil {
		inner = *mariadb.Status.GaleraRecovery
	} else {
		inner = mariadbv1alpha1.GaleraRecoveryStatus{
			State:         make(map[string]*agentgalera.GaleraState),
			Recovered:     make(map[string]*agentgalera.Bootstrap),
			BootstrapTime: nil,
		}
	}
	return &recoveryStatus{
		inner: &inner,
		mux:   &sync.RWMutex{},
	}
}
func (rs *recoveryStatus) galeraRecoveryStatus() *mariadbv1alpha1.GaleraRecoveryStatus {
	rs.mux.RLock()
	defer rs.mux.RUnlock()

	return rs.inner
}

func (rs *recoveryStatus) setState(pod string, state *agentgalera.GaleraState) {
	rs.mux.Lock()
	defer rs.mux.Unlock()

	rs.inner.State[pod] = state
}

func (rs *recoveryStatus) state(pod string) (*agentgalera.GaleraState, bool) {
	rs.mux.RLock()
	defer rs.mux.RUnlock()

	state, ok := rs.inner.State[pod]
	return state, ok
}

func (rs *recoveryStatus) setRecovered(pod string, bootstrap *agentgalera.Bootstrap) {
	rs.mux.Lock()
	defer rs.mux.Unlock()

	rs.inner.Recovered[pod] = bootstrap
}

func (rs *recoveryStatus) recovered(pod string) (*agentgalera.Bootstrap, bool) {
	rs.mux.RLock()
	defer rs.mux.RUnlock()

	bootstrap, ok := rs.inner.Recovered[pod]
	return bootstrap, ok
}

func (rs *recoveryStatus) reset() {
	rs.mux.Lock()
	defer rs.mux.Unlock()

	rs.inner = &mariadbv1alpha1.GaleraRecoveryStatus{
		State:         make(map[string]*agentgalera.GaleraState),
		Recovered:     make(map[string]*agentgalera.Bootstrap),
		BootstrapTime: nil,
	}
}

func (rs *recoveryStatus) setBootstrapping() {
	rs.mux.Lock()
	defer rs.mux.Unlock()

	now := metav1.NewTime(time.Now())
	rs.inner.BootstrapTime = &now
}

func (rs *recoveryStatus) isBootstrapping() bool {
	rs.mux.RLock()
	defer rs.mux.RUnlock()

	return rs.inner.BootstrapTime != nil
}

func (rs *recoveryStatus) bootstrapTimeout(mdb *mariadbv1alpha1.MariaDB) bool {
	if !rs.isBootstrapping() {
		return false
	}
	rs.mux.RLock()
	defer rs.mux.RUnlock()

	deadline := rs.inner.BootstrapTime.Time.Add(mdb.Spec.Galera.Recovery.ClusterBootstrapTimeoutOrDefault())
	return time.Now().After(deadline)
}

func (rs *recoveryStatus) isComplete(pods []corev1.Pod) bool {
	if rs.safeToBootstrap() != nil {
		return true
	}
	rs.mux.RLock()
	defer rs.mux.RUnlock()

	for _, p := range pods {
		if rs.inner.State[p.Name] == nil || rs.inner.Recovered[p.Name] == nil {
			return false
		}
	}
	return true
}

func (rs *recoveryStatus) safeToBootstrap() *bootstrapSource {
	rs.mux.RLock()
	defer rs.mux.RUnlock()

	for k, v := range rs.inner.State {
		if v.SafeToBootstrap && v.Seqno != -1 {
			return &bootstrapSource{
				bootstrap: &agentgalera.Bootstrap{
					UUID:  v.UUID,
					Seqno: v.Seqno,
				},
				pod: k,
			}
		}
	}
	return nil
}

func (rs *recoveryStatus) bootstrapSource(pods []corev1.Pod) (*bootstrapSource, error) {
	if source := rs.safeToBootstrap(); source != nil {
		return source, nil
	}

	rs.mux.RLock()
	defer rs.mux.RUnlock()
	var currentSoure agentgalera.GaleraRecoverer
	var currentPod string

	for _, p := range pods {
		state := rs.inner.State[p.Name]
		recovered := rs.inner.Recovered[p.Name]
		if state != nil && state.GetSeqno() != -1 && state.Compare(currentSoure) >= 0 {
			currentSoure = state
			currentPod = p.Name
		}
		if recovered != nil && recovered.GetSeqno() != -1 && recovered.Compare(currentSoure) >= 0 {
			currentSoure = state
			currentPod = p.Name
		}
	}
	if currentSoure == nil {
		return nil, errors.New("bootstrap source not found")
	}
	return &bootstrapSource{
		bootstrap: &agentgalera.Bootstrap{
			UUID:  currentSoure.GetUUID(),
			Seqno: currentSoure.GetSeqno(),
		},
		pod: currentPod,
	}, nil
}
