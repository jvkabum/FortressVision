package main

import (
	"reflect"
	"testing"

	"FortressVision/shared/pkg/dfproto"
	"FortressVision/shared/util"
)

func TestRegionZLevelsPrioritizeCenter(t *testing.T) {
	got := regionZLevels(100, 2, 1)
	want := []int32{100, 99, 101, 98}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("levels = %v, want %v", got, want)
	}
}

func TestResponseBlockOriginUsesTileCoordinates(t *testing.T) {
	got := responseBlockOrigin(dfproto.MapBlock{MapX: 74, MapY: 84, MapZ: 139})
	want := util.DFCoord{X: 64, Y: 80, Z: 139}
	if got != want {
		t.Fatalf("response block origin = %v, want %v", got, want)
	}
}

func TestRegionZLevelsClampInvalidRanges(t *testing.T) {
	got := regionZLevels(0, -4, 99)
	if len(got) != 33 || got[0] != 0 || got[len(got)-1] != 32 {
		t.Fatalf("unexpected clamped levels: len=%d values=%v", len(got), got)
	}
}
