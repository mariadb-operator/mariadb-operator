package galera

import (
	"errors"
	"fmt"
	"sync"
	"time"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/pkg/galera/recovery"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

type recoveryStatus struct {
	inner mariadbv1alpha1.GaleraRecoveryStatus
	mux   *sync.RWMutex
}

type bootstrapSource struct {
	bootstrap *recovery.Bootstrap
	pod       *corev1.Pod
}

func (b *bootstrapSource) String() string {
	return fmt.Sprintf(
		"{ bootstrap: { UUID: %s, seqno: %d }, pod: %s }",
		b.bootstrap.UUID,
		b.bootstrap.Seqno,
		b.pod.Name,
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

func (rs *recoveryStatus) safeToBootstrap(pods []corev1.Pod) (*bootstrapSource, error) {
	rs.mux.RLock()
	defer rs.mux.RUnlock()

	for k, v := range rs.inner.State {
		if v.SafeToBootstrap && v.Seqno != -1 {
			for _, p := range pods {
				if k == p.Name {
					return &bootstrapSource{
						bootstrap: &recovery.Bootstrap{
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
	return nil, errors.New("no Pods safe to bootstrap were found")
}

func (rs *recoveryStatus) isComplete(pods []corev1.Pod) bool {
	if len(pods) == 0 {
		return false
	}
	rs.mux.RLock()
	defer rs.mux.RUnlock()

	for _, p := range pods {
		state := rs.inner.State[p.Name]
		recovered := rs.inner.Recovered[p.Name]
		if (state != nil && state.Seqno != -1) || (recovered != nil && recovered.Seqno != -1) {
			continue
		}
		return false
	}
	return true
}

func (rs *recoveryStatus) bootstrapSource(pods []corev1.Pod) (*bootstrapSource, error) {
	if source, err := rs.safeToBootstrap(pods); source != nil && err == nil {
		return source, nil
	}
	if !rs.isComplete(pods) {
		return nil, errors.New("recovery status not completed")
	}

	rs.mux.RLock()
	defer rs.mux.RUnlock()
	var currentSoure recovery.GaleraRecoverer
	var currentPod corev1.Pod

	for _, p := range pods {
		state := rs.inner.State[p.Name]
		recovered := rs.inner.Recovered[p.Name]
		if state != nil && state.GetSeqno() != -1 && state.Compare(currentSoure) >= 0 {
			currentSoure = state
			currentPod = p
		}
		if recovered != nil && recovered.GetSeqno() != -1 && recovered.Compare(currentSoure) >= 0 {
			currentSoure = recovered
			currentPod = p
		}
	}
	if currentSoure == nil {
		return nil, errors.New("bootstrap source not found")
	}
	return &bootstrapSource{
		bootstrap: &recovery.Bootstrap{
			UUID:  currentSoure.GetUUID(),
			Seqno: currentSoure.GetSeqno(),
		},
		pod: &currentPod,
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
