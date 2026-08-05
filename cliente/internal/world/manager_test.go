package world

import (
	"testing"

	"FortressVision/shared/mapdata"
	"FortressVision/shared/proto/fvnet"
	"FortressVision/shared/util"
	"google.golang.org/protobuf/proto"
)

func TestRegionWindowContainsIntersectingChunk(t *testing.T) {
	region := regionWindow{
		center:     util.DFCoord{X: 32, Y: 32, Z: 10},
		radius:     16,
		levelsDown: 1,
		levelsUp:   2,
	}

	for _, origin := range []util.DFCoord{
		{X: 16, Y: 16, Z: 9},
		{X: 48, Y: 48, Z: 12},
	} {
		if !region.contains(origin) {
			t.Fatalf("region should contain intersecting chunk %v", origin)
		}
	}
	for _, origin := range []util.DFCoord{
		{X: 0, Y: 16, Z: 10},
		{X: 16, Y: 16, Z: 8},
		{X: 16, Y: 16, Z: 13},
	} {
		if region.contains(origin) {
			t.Fatalf("region should reject chunk %v", origin)
		}
	}
}

func TestSetActiveRegionPrunesOldChunksAndReportsRemoval(t *testing.T) {
	m := NewManager()
	inside := util.DFCoord{X: 16, Y: 16, Z: 10}
	outside := util.DFCoord{X: 96, Y: 16, Z: 10}
	m.Store.Chunks[inside] = &mapdata.Chunk{Origin: inside}
	m.Store.Chunks[outside] = &mapdata.Chunk{Origin: outside}
	var removed []util.DFCoord
	m.OnMapChunkRemoved = func(origin util.DFCoord) {
		removed = append(removed, origin)
	}

	m.SetActiveRegion(util.DFCoord{X: 32, Y: 32, Z: 10}, 16, 0, 0)

	if _, ok := m.Store.Chunks[inside]; !ok {
		t.Fatal("chunk intersecting active region was pruned")
	}
	if _, ok := m.Store.Chunks[outside]; ok {
		t.Fatal("old chunk remained after active-region update")
	}
	if len(removed) != 1 || removed[0] != outside {
		t.Fatalf("removed chunks = %v, want [%v]", removed, outside)
	}
}

func TestRequestRegionRejectsLateChunkOutsideActiveWindow(t *testing.T) {
	m := NewManager()
	m.RequestRegion(func(_ fvnet.Envelope_Type, _ proto.Message) {}, util.DFCoord{X: 32, Y: 32, Z: 10}, 16, 0, 0)

	if m.acceptsOrigin(util.DFCoord{X: 96, Y: 16, Z: 10}) {
		t.Fatal("late chunk outside the requested region was accepted")
	}
}
