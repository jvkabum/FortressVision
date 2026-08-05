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
		SizeCurrent:     120,
		SizeBase:        100,
		IsSoldier:       true,
		Pos:             util.DFCoord{X: 10, Y: 20, Z: 3},
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
		!got.Units[0].IsSoldier {
		t.Fatalf("round trip mismatch: got %#v, want %#v", got.Units, want)
	}
}
