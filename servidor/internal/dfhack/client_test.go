package dfhack

import (
	"testing"

	"FortressVision/shared/pkg/dfproto"
)

func TestAttachEngravingsToBlocks(t *testing.T) {
	list := &dfproto.BlockList{
		MapBlocks: []dfproto.MapBlock{
			{MapX: 16, MapY: 32, MapZ: 9},
			{MapX: 32, MapY: 32, MapZ: 9},
		},
		Engravings: []dfproto.Engraving{
			{Pos: dfproto.Coord{X: 17, Y: 47, Z: 9}},
			{Pos: dfproto.Coord{X: 32, Y: 32, Z: 9}},
		},
		OceanWaves: []dfproto.Wave{
			{Pos: dfproto.Coord{X: 18, Y: 34, Z: 9}, Dest: dfproto.Coord{X: 19, Y: 34, Z: 9}},
		},
	}

	attachEngravingsToBlocks(list)
	if len(list.MapBlocks[0].Engravings) != 1 {
		t.Fatalf("first block received %d engravings, want 1", len(list.MapBlocks[0].Engravings))
	}
	if len(list.MapBlocks[1].Engravings) != 1 {
		t.Fatalf("second block received %d engravings, want 1", len(list.MapBlocks[1].Engravings))
	}
	if len(list.MapBlocks[0].OceanWaves) != 1 {
		t.Fatalf("first block received %d waves, want 1", len(list.MapBlocks[0].OceanWaves))
	}
	if len(list.MapBlocks[1].OceanWaves) != 0 {
		t.Fatalf("second block received %d waves, want 0", len(list.MapBlocks[1].OceanWaves))
	}
}
