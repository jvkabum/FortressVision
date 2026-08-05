package fvnet

import (
	"testing"

	"google.golang.org/protobuf/proto"
)

func TestClientRequestRegionCarriesVerticalLevels(t *testing.T) {
	want := &ClientRequestRegion{
		CenterX:    10,
		CenterY:    20,
		CenterZ:    30,
		Radius:     64,
		LevelsDown: 5,
		LevelsUp:   2,
	}
	data, err := proto.Marshal(want)
	if err != nil {
		t.Fatalf("marshal region request: %v", err)
	}
	var got ClientRequestRegion
	if err := proto.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal region request: %v", err)
	}
	if got.CenterX != want.CenterX || got.CenterY != want.CenterY || got.CenterZ != want.CenterZ ||
		got.Radius != want.Radius || got.LevelsDown != want.LevelsDown || got.LevelsUp != want.LevelsUp {
		t.Fatalf("region request mismatch: got %+v, want %+v", got, *want)
	}
}
