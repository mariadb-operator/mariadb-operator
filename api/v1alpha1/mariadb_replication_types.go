/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	"fmt"
	"time"

	"github.com/mariadb-operator/mariadb-operator/pkg/statefulset"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	defaultReplicaConnTimeout = 10 * time.Second
	defaultReplicaSyncTimeout = 10 * time.Second
)

type WaitPoint string

const (
	WaitPointAfterSync   WaitPoint = "AfterSync"
	WaitPointAfterCommit WaitPoint = "AfterCommit"
)

func (w WaitPoint) Validate() error {
	switch w {
	case WaitPointAfterSync, WaitPointAfterCommit:
		return nil
	default:
		return fmt.Errorf("invalid WaitPoint: %v", w)
	}
}

func (w WaitPoint) MariaDBFormat() (string, error) {
	switch w {
	case WaitPointAfterSync:
		return "AFTER_SYNC", nil
	case WaitPointAfterCommit:
		return "AFTER_COMMIT", nil
	default:
		return "", fmt.Errorf("invalid WaitPoint: %v", w)
	}
}

type Gtid string

const (
	GtidCurrentPos Gtid = "CurrentPos"
	GtidSlavePos   Gtid = "SlavePos"
)

func (g Gtid) Validate() error {
	switch g {
	case GtidCurrentPos, GtidSlavePos:
		return nil
	default:
		return fmt.Errorf("invalid Gtid: %v", g)
	}
}

func (g Gtid) MariaDBFormat() (string, error) {
	switch g {
	case GtidCurrentPos:
		return "current_pos", nil
	case GtidSlavePos:
		return "slave_pos", nil
	default:
		return "", fmt.Errorf("invalid Gtid: %v", g)
	}
}

type PrimaryReplication struct {
	// +kubebuilder:default=0
	PodIndex int `json:"podIndex,omitempty"`
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=true
	AutomaticFailover bool `json:"automaticFailover"`

	Service *Service `json:"service,omitempty"`

	Connection *ConnectionTemplate `json:"connection,omitempty"`
}

type ReplicaReplication struct {
	// +kubebuilder:default=AfterCommit
	WaitPoint *WaitPoint `json:"waitPoint,omitempty"`
	// +kubebuilder:default=CurrentPos
	Gtid *Gtid `json:"gtid,omitempty"`

	ConnectionTimeout *metav1.Duration `json:"connectionTimeout,omitempty"`
	// +kubebuilder:default=10
	ConnectionRetries int `json:"connectionRetries,omitempty"`

	SyncTimeout *metav1.Duration `json:"syncTimeout,omitempty"`
}

func (r *ReplicaReplication) Validate() error {
	if r.WaitPoint != nil {
		if err := r.WaitPoint.Validate(); err != nil {
			return fmt.Errorf("invalid WaitPoint: %v", err)
		}
	}
	if r.Gtid != nil {
		if err := r.Gtid.Validate(); err != nil {
			return fmt.Errorf("invalid GTID: %v", err)
		}
	}
	return nil
}

func (r *ReplicaReplication) ConnectionTimeoutOrDefault() time.Duration {
	if r.ConnectionTimeout != nil {
		return r.ConnectionTimeout.Duration
	}
	return defaultReplicaConnTimeout
}

func (r *ReplicaReplication) SyncTimeoutOrDefault() time.Duration {
	if r.SyncTimeout != nil {
		return r.SyncTimeout.Duration
	}
	return defaultReplicaSyncTimeout
}

type Replication struct {
	// +kubebuilder:validation:Required
	Primary PrimaryReplication `json:"primary"`
	// +kubebuilder:default={}
	Replica ReplicaReplication `json:"replica,omitempty"`
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=true
	SyncBinlog bool `json:"syncBinlog"`
}

func (m *MariaDB) IsConfiguringReplication() bool {
	return meta.IsStatusConditionFalse(m.Status.Conditions, ConditionTypeReplicationConfigured)
}

func (m *MariaDB) IsSwitchingPrimary() bool {
	return meta.IsStatusConditionFalse(m.Status.Conditions, ConditionTypePrimarySwitched)
}

func (s *MariaDBStatus) UpdateCurrentPrimary(mariadb *MariaDB, index int) {
	s.CurrentPrimaryPodIndex = &index
	primaryPod := statefulset.PodName(mariadb.ObjectMeta, index)
	s.CurrentPrimary = &primaryPod
}

func (s *MariaDBStatus) UpdateCurrentPrimaryName(name string) {
	s.CurrentPrimary = &name
}
