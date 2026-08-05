package dfhack

import (
	"testing"

	"FortressVision/shared/pkg/dfproto"
	"FortressVision/shared/util"
)

func TestCoordinateNormalizerUsesLocalXYAndAbsoluteZ(t *testing.T) {
	normalizer := NewCoordinateNormalizer(&dfproto.MapInfo{
		BlockPosX: 2,
		BlockPosY: 3,
		BlockPosZ: 126,
	})

	req := normalizer.ToRemoteBlockRequest(4, 5, 165, 7, 8, 166, 20)
	if req.MinX != 2 || req.MaxX != 5 || req.MinY != 2 || req.MaxY != 5 || req.MinZ != 165 || req.MaxZ != 166 {
		t.Fatalf("request frame is wrong: %+v", req)
	}

	list := &dfproto.BlockList{
		MapBlocks: []dfproto.MapBlock{{
			MapX:  16,
			MapY:  32,
			MapZ:  39,
			TreeZ: []int32{39},
			Buildings: []dfproto.BuildingInstance{{
				PosZMin: 39,
				PosZMax: 40,
				Items:   []dfproto.BuildingItem{{Item: &dfproto.Item{Pos: dfproto.Coord{Z: 39}}}},
			}},
			Flows: []dfproto.FlowInfo{{
				Pos:  dfproto.Coord{Z: 39},
				Dest: dfproto.Coord{Z: 40},
			}},
			Items:  []dfproto.Item{{Pos: dfproto.Coord{Z: 39}}},
			Plants: []dfproto.PlantDetail{{Pos: dfproto.Coord{Z: 39}}},
		}},
		Engravings: []dfproto.Engraving{{Pos: dfproto.Coord{Z: 39}}},
		OceanWaves: []dfproto.Wave{{Pos: dfproto.Coord{Z: 39}, Dest: dfproto.Coord{Z: 40}}},
	}
	normalizer.NormalizeBlockList(list)

	block := list.MapBlocks[0]
	if block.MapZ != 39 || block.TreeZ[0] != 39 {
		t.Fatalf("block coordinates were changed: %+v", block)
	}
	if block.Buildings[0].PosZMin != 39 || block.Buildings[0].PosZMax != 40 || block.Buildings[0].Items[0].Item.Pos.Z != 39 {
		t.Fatalf("building coordinates were changed: %+v", block.Buildings[0])
	}
	if block.Flows[0].Pos.Z != 39 || block.Flows[0].Dest.Z != 40 || block.Items[0].Pos.Z != 39 || block.Plants[0].Pos.Z != 39 {
		t.Fatalf("entity coordinates were changed: %+v", block)
	}
	if list.Engravings[0].Pos.Z != 39 || list.OceanWaves[0].Dest.Z != 40 {
		t.Fatalf("envelope coordinates were changed: %+v", list)
	}
}

func TestCoordinateNormalizerSamplesViewAndCursor(t *testing.T) {
	normalizer := NewCoordinateNormalizer(&dfproto.MapInfo{BlockPosZ: 126})
	view := &dfproto.ViewInfo{
		ViewPosX:   100,
		ViewPosY:   200,
		ViewSizeX:  10,
		ViewSizeY:  6,
		ViewPosZ:   39,
		CursorPosX: 104,
		CursorPosY: 203,
		CursorPosZ: 39,
	}
	sample := normalizer.SampleViewInfo(view)
	if sample.RawViewCenter != (util.DFCoord{X: 105, Y: 203, Z: 39}) {
		t.Fatalf("unexpected raw view center: %v", sample.RawViewCenter)
	}
	if sample.AbsoluteCursor != (util.DFCoord{X: 104, Y: 203, Z: 39}) {
		t.Fatalf("unexpected absolute cursor: %v", sample.AbsoluteCursor)
	}
	normalizer.NormalizeViewInfo(view)
	if view.ViewPosZ != 39 || view.CursorPosZ != 39 {
		t.Fatalf("view coordinates were changed: %+v", view)
	}
}

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
