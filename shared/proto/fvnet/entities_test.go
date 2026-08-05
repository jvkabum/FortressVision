package fvnet

import (
	"testing"

	"FortressVision/shared/mapdata"
	"FortressVision/shared/pkg/dfproto"
	"FortressVision/shared/util"
)

func TestCreatureUpdateMessageRoundTrip(t *testing.T) {
	want := []mapdata.UnitInstance{{
		ID:              42,
		Name:            "Urist",
		ProfessionColor: dfproto.ColorDefinition{Red: 200, Green: 80, Blue: 30},
		Appearance:      dfproto.UnitAppearance{SizeModifier: 2},
		Inventory: []dfproto.InventoryItem{{
			Mode: 1,
			Item: dfproto.Item{Type: dfproto.MatPair{MatType: 4, MatIndex: 7}, StackSize: 1},
		}},
		Wounds:      []dfproto.UnitWound{{SeveredPart: true}},
		Facing:      dfproto.Coord{X: 1, Y: 0, Z: 0},
		SizeCurrent: 120,
		SizeBase:    100,
		IsSoldier:   true,
		Pos:         util.DFCoord{X: 10, Y: 20, Z: 3},
		SubPos: util.Vector3{
			X: 0.25,
			Y: 0.5,
			Z: 0.75,
		},
	}}

	payload, err := (&CreatureUpdateMessage{Units: want}).Marshal()
	if err != nil {
		t.Fatalf("marshal creature update: %v", err)
	}

	var got CreatureUpdateMessage
	if err := got.Unmarshal(payload); err != nil {
		t.Fatalf("unmarshal creature update: %v", err)
	}
	if len(got.Units) != 1 || got.Units[0].ID != want[0].ID || got.Units[0].Pos != want[0].Pos ||
		got.Units[0].ProfessionColor != want[0].ProfessionColor || got.Units[0].SizeCurrent != want[0].SizeCurrent ||
		!got.Units[0].IsSoldier || len(got.Units[0].Inventory) != 1 ||
		got.Units[0].Inventory[0].Item.Type != want[0].Inventory[0].Item.Type ||
		len(got.Units[0].Wounds) != 1 || !got.Units[0].Wounds[0].SeveredPart || got.Units[0].Facing != want[0].Facing {
		t.Fatalf("round trip mismatch: got %#v, want %#v", got.Units, want)
	}
}
