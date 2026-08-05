package mapdata

import (
	"testing"

	"FortressVision/shared/pkg/dfproto"
	"FortressVision/shared/util"
)

func TestStoreSingleBlockDoesNotReportUnchangedEntitiesAsChanges(t *testing.T) {
	store := NewMapDataStore()
	block := dfproto.MapBlock{
		MapX: 0,
		MapY: 0,
		MapZ: 0,
		Buildings: []dfproto.BuildingInstance{{
			Index: 7,
			Items: []dfproto.BuildingItem{{Item: &dfproto.Item{ID: 8}}},
		}},
		Items:       []dfproto.Item{{ID: 9, StackSize: 2}},
		SpatterPile: []dfproto.SpatterPile{{Spatters: []dfproto.Spatter{{Amount: 3}}}},
		Engravings:  []dfproto.Engraving{{Pos: dfproto.Coord{X: 1, Y: 1, Z: 0}}},
	}

	if first := store.StoreSingleBlock(&block); first == NoChange {
		t.Fatalf("first block load must report a change")
	}
	if second := store.StoreSingleBlock(&block); second != NoChange {
		t.Fatalf("identical entity snapshot reported change %v", second)
	}
}

func TestMarkAsEmptyClearsExistingChunkAndReportsTransition(t *testing.T) {
	store := NewMapDataStore()
	block := dfproto.MapBlock{
		MapX:  0,
		MapY:  0,
		MapZ:  12,
		Tiles: []int32{1},
		Items: []dfproto.Item{{ID: 11}},
	}
	store.StoreSingleBlock(&block)

	origin := util.NewDFCoord(block.MapX, block.MapY, block.MapZ).BlockCoord()
	if !store.MarkAsEmpty(origin) {
		t.Fatal("expected a non-empty chunk to report an empty transition")
	}
	chunk, ok := store.GetChunk(origin)
	if !ok || !chunk.IsEmpty {
		t.Fatal("chunk was not marked empty")
	}
	if chunk.Tiles[0][0] != nil || len(chunk.Items) != 0 {
		t.Fatal("empty transition left stale chunk data")
	}
	if store.MarkAsEmpty(origin) {
		t.Fatal("repeating an empty transition should be a no-op")
	}
}
