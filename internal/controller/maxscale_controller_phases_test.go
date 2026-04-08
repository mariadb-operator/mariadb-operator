package controller

import "testing"

func TestMaxScaleReconcilePhasesBootstrapBeforeReadyGate(t *testing.T) {
	phases := (&MaxScaleReconciler{}).reconcilePhases()
	phaseIndex := make(map[string]int, len(phases))

	for i, phase := range phases {
		phaseIndex[phase.name] = i
	}

	required := []string{
		"Pod Clients",
		"Admin",
		"Init",
		"Sync",
		"StatefulSet Ready",
		"Client",
	}
	for _, name := range required {
		if _, ok := phaseIndex[name]; !ok {
			t.Fatalf("expected phase %q to be present", name)
		}
	}

	if phaseIndex["Pod Clients"] >= phaseIndex["Admin"] {
		t.Fatalf("expected Pod Clients phase to run before Admin")
	}
	if phaseIndex["Admin"] >= phaseIndex["Init"] {
		t.Fatalf("expected Admin phase to run before Init")
	}
	if phaseIndex["Init"] >= phaseIndex["Sync"] {
		t.Fatalf("expected Init phase to run before Sync")
	}
	if phaseIndex["Sync"] >= phaseIndex["StatefulSet Ready"] {
		t.Fatalf("expected Sync phase to run before StatefulSet Ready")
	}
	if phaseIndex["StatefulSet Ready"] >= phaseIndex["Client"] {
		t.Fatalf("expected StatefulSet Ready phase to run before Client")
	}
}
