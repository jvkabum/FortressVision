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
	}

	attachEngravingsToBlocks(list)
	if len(list.MapBlocks[0].Engravings) != 1 {
		t.Fatalf("first block received %d engravings, want 1", len(list.MapBlocks[0].Engravings))
	}
	if len(list.MapBlocks[1].Engravings) != 1 {
		t.Fatalf("second block received %d engravings, want 1", len(list.MapBlocks[1].Engravings))
	}
}
