package dfhack

import (
	"FortressVision/shared/pkg/dfproto"
	"FortressVision/shared/util"
)

// CoordinateNormalizer is the boundary between RemoteFortressReader and the
// rest of FortressVision.
//
// RemoteFortressReader uses map-local block indices for requests and for the Z
// component of returned map data. MapX/MapY in MapBlock are tile coordinates,
// while MapInfo.BlockPosZ is the absolute Z origin of the map. FortressVision
// uses absolute DF tile coordinates after this boundary.
type CoordinateNormalizer struct {
	blockOrigin util.DFCoord
}

// ViewCoordinateSample keeps both frames available for diagnostics. It is
// useful when comparing the DFHack cursor with Stonesense or the in-game HUD.
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
		blockOrigin: util.NewDFCoord(info.BlockPosX, info.BlockPosY, info.BlockPosZ),
	}
}

// ToRemoteBlockRequest converts absolute block indices to the local frame
// expected by RemoteFortressReader.
func (n CoordinateNormalizer) ToRemoteBlockRequest(minX, minY, minZ, maxX, maxY, maxZ, blocksNeeded int32) *dfproto.BlockRequest {
	return &dfproto.BlockRequest{
		BlocksNeeded: blocksNeeded,
		MinX:         minX - n.blockOrigin.X,
		MaxX:         maxX - n.blockOrigin.X,
		MinY:         minY - n.blockOrigin.Y,
		MaxY:         maxY - n.blockOrigin.Y,
		MinZ:         minZ - n.blockOrigin.Z,
		MaxZ:         maxZ - n.blockOrigin.Z,
		ForceReload:  true,
	}
}

// NormalizeViewInfo converts the camera and cursor Z values to the absolute
// DF frame. X and Y are already absolute tile coordinates in RFR ViewInfo.
func (n CoordinateNormalizer) NormalizeViewInfo(view *dfproto.ViewInfo) {
	if view == nil {
		return
	}
	view.ViewPosZ += n.blockOrigin.Z
	view.CursorPosZ += n.blockOrigin.Z
}

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
	absoluteCenter := rawCenter
	absoluteCenter.Z += n.blockOrigin.Z
	absoluteCursor := rawCursor
	absoluteCursor.Z += n.blockOrigin.Z
	return ViewCoordinateSample{
		RawViewCenter:      rawCenter,
		AbsoluteViewCenter: absoluteCenter,
		RawCursor:          rawCursor,
		AbsoluteCursor:     absoluteCursor,
	}
}

func (n CoordinateNormalizer) normalizeCoord(coord *dfproto.Coord) {
	if coord != nil {
		coord.Z += n.blockOrigin.Z
	}
}

func (n CoordinateNormalizer) normalizeItem(item *dfproto.Item) {
	if item == nil {
		return
	}
	n.normalizeCoord(&item.Pos)
}

func (n CoordinateNormalizer) normalizeBuilding(building *dfproto.BuildingInstance) {
	if building == nil {
		return
	}
	building.PosZMin += n.blockOrigin.Z
	building.PosZMax += n.blockOrigin.Z
	for i := range building.Items {
		n.normalizeItem(building.Items[i].Item)
	}
}

func (n CoordinateNormalizer) normalizeBlock(block *dfproto.MapBlock) {
	if block == nil {
		return
	}
	block.MapZ += n.blockOrigin.Z
	for i := range block.TreeZ {
		block.TreeZ[i] += n.blockOrigin.Z
	}
	for i := range block.Buildings {
		n.normalizeBuilding(&block.Buildings[i])
	}
	for i := range block.Flows {
		n.normalizeCoord(&block.Flows[i].Pos)
		n.normalizeCoord(&block.Flows[i].Dest)
	}
	for i := range block.Items {
		n.normalizeItem(&block.Items[i])
	}
	for i := range block.Plants {
		n.normalizeCoord(&block.Plants[i].Pos)
	}
	for i := range block.Engravings {
		n.normalizeCoord(&block.Engravings[i].Pos)
	}
	for i := range block.OceanWaves {
		n.normalizeCoord(&block.OceanWaves[i].Pos)
		n.normalizeCoord(&block.OceanWaves[i].Dest)
	}
}

// NormalizeBlockList makes every positional field in a block response
// absolute before it reaches the map store or websocket layer.
func (n CoordinateNormalizer) NormalizeBlockList(list *dfproto.BlockList) {
	if list == nil {
		return
	}
	for i := range list.MapBlocks {
		n.normalizeBlock(&list.MapBlocks[i])
	}
	for i := range list.Engravings {
		n.normalizeCoord(&list.Engravings[i].Pos)
	}
	for i := range list.OceanWaves {
		n.normalizeCoord(&list.OceanWaves[i].Pos)
		n.normalizeCoord(&list.OceanWaves[i].Dest)
	}
}

// NormalizeUnitList applies the same absolute-Z rule to units and their
// carried items before they are copied into the world store.
func (n CoordinateNormalizer) NormalizeUnitList(list *dfproto.UnitList) {
	if list == nil {
		return
	}
	for i := range list.CreatureList {
		unit := &list.CreatureList[i]
		unit.PosZ += n.blockOrigin.Z
		for itemIndex := range unit.Inventory {
			n.normalizeItem(&unit.Inventory[itemIndex].Item)
		}
	}
}
