package dfhack

import (
	"FortressVision/shared/pkg/dfproto"
	"FortressVision/shared/util"
)

// CoordinateNormalizer is the explicit boundary between RemoteFortressReader
// and the rest of FortressVision.
//
// FortressVision keeps tile coordinates local to the loaded map in X/Y, while
// Z is the absolute DF level. GetBlockList callers use global block indices in
// X/Y, but RemoteFortressReader expects local block indices there. Its Z
// request and MapBlock.MapZ are already absolute. Item/unit/view coordinates
// are copied from DF structures in the same local-X/Y, absolute-Z frame.
//
// Keeping this boundary explicit prevents ad-hoc coordinate arithmetic from
// reappearing in callers when the protocol is inspected or changed later.
type CoordinateNormalizer struct {
	mapPosition util.DFCoord
}

// ViewCoordinateSample keeps both labels for diagnostics when comparing the
// DFHack cursor with Stonesense or the in-game HUD. Both frames are identical
// for the current RemoteFortressReader contract.
type ViewCoordinateSample struct {
	RawViewCenter      util.DFCoord
	AbsoluteViewCenter util.DFCoord
	RawCursor          util.DFCoord
	AbsoluteCursor     util.DFCoord
}

func NewCoordinateNormalizer(info *dfproto.MapInfo) CoordinateNormalizer {
	if info == nil {
		return CoordinateNormalizer{}
	}
	return CoordinateNormalizer{
		mapPosition: util.NewDFCoord(info.BlockPosX, info.BlockPosY, info.BlockPosZ),
	}
}

// ToRemoteBlockRequest converts global X/Y block indices to the local block
// indices expected by RemoteFortressReader. Z must remain absolute.
func (n CoordinateNormalizer) ToRemoteBlockRequest(minX, minY, minZ, maxX, maxY, maxZ, blocksNeeded int32) *dfproto.BlockRequest {
	return &dfproto.BlockRequest{
		BlocksNeeded: blocksNeeded,
		MinX:         minX - n.mapPosition.X,
		MaxX:         maxX - n.mapPosition.X,
		MinY:         minY - n.mapPosition.Y,
		MaxY:         maxY - n.mapPosition.Y,
		MinZ:         minZ,
		MaxZ:         maxZ,
		ForceReload:  true,
	}
}

// NormalizeViewInfo is intentionally a no-op. ViewInfo is already absolute in
// RemoteFortressReader; adding MapInfo.BlockPosZ here caused the HUD elevation
// mismatch and moved the camera away from the DF map.
func (n CoordinateNormalizer) NormalizeViewInfo(view *dfproto.ViewInfo) {}

func (n CoordinateNormalizer) SampleViewInfo(view *dfproto.ViewInfo) ViewCoordinateSample {
	if view == nil {
		return ViewCoordinateSample{}
	}
	rawCenter := util.NewDFCoord(
		view.ViewPosX+view.ViewSizeX/2,
		view.ViewPosY+view.ViewSizeY/2,
		view.ViewPosZ,
	)
	rawCursor := util.NewDFCoord(view.CursorPosX, view.CursorPosY, view.CursorPosZ)
	return ViewCoordinateSample{
		RawViewCenter:      rawCenter,
		AbsoluteViewCenter: rawCenter,
		RawCursor:          rawCursor,
		AbsoluteCursor:     rawCursor,
	}
}

// NormalizeBlockList documents the protocol boundary and deliberately leaves
// the response untouched. Positional fields in a BlockList are already in DF's
// absolute frame. TreeX/TreeY/TreeZ are relative tree-shape data and must not
// be treated as world coordinates.
func (n CoordinateNormalizer) NormalizeBlockList(list *dfproto.BlockList) {}

// NormalizeUnitList is intentionally a no-op because unit positions and
// carried-item positions are copied from absolute DF coordinates by RFR.
func (n CoordinateNormalizer) NormalizeUnitList(list *dfproto.UnitList) {}
