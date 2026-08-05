package dfproto

import "testing"

func TestUnitDefinitionOptionalValidityDefaultsToDisplayable(t *testing.T) {
	var unit UnitDefinition
	if err := unit.Unmarshal([]byte{0x08, 0x07}); err != nil {
		t.Fatalf("unmarshal unit without isValid: %v", err)
	}
	if unit.IsValidSet {
		t.Fatalf("isValid should be marked absent")
	}
	if !unit.IsValidForDisplay() {
		t.Fatalf("unit without optional isValid should remain displayable")
	}
}

func TestUnitDefinitionExplicitInvalidValidityIsHonored(t *testing.T) {
	var unit UnitDefinition
	if err := unit.Unmarshal([]byte{0x08, 0x07, 0x10, 0x00}); err != nil {
		t.Fatalf("unmarshal invalid unit: %v", err)
	}
	if !unit.IsValidSet {
		t.Fatalf("isValid should be marked present")
	}
	if unit.IsValidForDisplay() {
		t.Fatalf("explicitly invalid unit should not be displayable")
	}
}
