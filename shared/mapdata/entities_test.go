package mapdata

import "testing"

func TestUnitValidityDoesNotHideNonLifecycleFlags(t *testing.T) {
	unit := UnitInstance{Flags1: 0x01 << 3}
	if !unit.IsValid() {
		t.Fatalf("non-lifecycle flags must not hide a live unit")
	}
}

func TestUnitValidityHonorsLifecycleState(t *testing.T) {
	dead := UnitInstance{IsDead: true}
	if dead.IsValid() {
		t.Fatalf("dead unit must not be rendered")
	}
	hidden := UnitInstance{IsHidden: true}
	if hidden.IsValid() {
		t.Fatalf("hidden unit must not be rendered")
	}
}

func TestPopulationCountUsesCurrentUnitSnapshot(t *testing.T) {
	store := NewMapDataStore()
	store.ReplaceUnits([]UnitInstance{
		{ID: 1},
		{ID: 2, IsDead: true},
		{ID: 3, IsHidden: true},
	})
	if got := store.PopulationCount(); got != 1 {
		t.Fatalf("population count = %d, want 1", got)
	}
}
