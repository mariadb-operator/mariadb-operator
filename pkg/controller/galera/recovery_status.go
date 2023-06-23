package galera

import (
	"errors"
	"fmt"
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
	pod       *corev1.Pod
}

func newRecoveryStatus(mariadb *mariadbv1alpha1.MariaDB) *recoveryStatus {
	var inner mariadbv1alpha1.GaleraRecoveryStatus
	if mariadb.Status.GaleraRecovery != nil {
		if mariadb.Status.GaleraRecovery.State != nil {
			inner.State = mariadb.Status.GaleraRecovery.State
		}
		if mariadb.Status.GaleraRecovery.Recovered != nil {
			inner.Recovered = mariadb.Status.GaleraRecovery.Recovered
		}
		if mariadb.Status.GaleraRecovery.BootstrapTime != nil {
			inner.BootstrapTime = mariadb.Status.GaleraRecovery.BootstrapTime
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

	if rs.inner.State == nil {
		rs.inner.State = make(map[string]*agentgalera.GaleraState)
	}
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

	if rs.inner.Recovered == nil {
		rs.inner.Recovered = make(map[string]*agentgalera.Bootstrap)
	}
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

	rs.inner = nil
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
	if source, err := rs.safeToBootstrap(pods); source != nil && err == nil {
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

func (rs *recoveryStatus) safeToBootstrap(pods []corev1.Pod) (*bootstrapSource, error) {
	rs.mux.RLock()
	defer rs.mux.RUnlock()

	for k, v := range rs.inner.State {
		if v.SafeToBootstrap && v.Seqno != -1 {
			for _, p := range pods {
				if k == p.Name {
					return &bootstrapSource{
						bootstrap: &agentgalera.Bootstrap{
							UUID:  v.UUID,
							Seqno: v.Seqno,
						},
						pod: &p,
					}, nil
				}
			}
			return nil, fmt.Errorf("Pod '%s' is safe to boostrap but it couldn't be found on the argument list", k)
		}
	}
	return nil, nil
}

func (rs *recoveryStatus) bootstrapSource(pods []corev1.Pod) (*bootstrapSource, error) {
	source, err := rs.safeToBootstrap(pods)
	if err != nil {
		return nil, fmt.Errorf("error finding safe source to bootstrap: %v", err)
	}
	if source != nil {
		return source, nil
	}

	rs.mux.RLock()
	defer rs.mux.RUnlock()
	var currentSoure agentgalera.GaleraRecoverer
	var currentPod corev1.Pod

	for _, p := range pods {
		state := rs.inner.State[p.Name]
		recovered := rs.inner.Recovered[p.Name]
		if state != nil && state.GetSeqno() != -1 && state.Compare(currentSoure) >= 0 {
			currentSoure = state
			currentPod = p
		}
		if recovered != nil && recovered.GetSeqno() != -1 && recovered.Compare(currentSoure) >= 0 {
			currentSoure = state
			currentPod = p
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
		pod: &currentPod,
	}, nil
}
