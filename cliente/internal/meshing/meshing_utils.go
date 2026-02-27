package meshing

import (
	"FortressVision/shared/mapdata"
	"FortressVision/shared/pkg/dfproto"
	"FortressVision/shared/util"
	"math"
)

func (m *BlockMesher) calculateAOMask(coord util.DFCoord, face util.Directions, data *mapdata.MapDataStore) uint32 {
	var mask uint32
	getBit := func(dir util.Directions) uint32 {
		if m.isSolidAO(coord, dir, data) {
			return 1
		}
		return 0
	}

	switch face {
	case util.DirUp, util.DirDown:
		mask |= getBit(util.DirNorth) << 0
		mask |= getBit(util.DirSouth) << 1
		mask |= getBit(util.DirWest) << 2
		mask |= getBit(util.DirEast) << 3
	case util.DirNorth, util.DirSouth:
		mask |= getBit(util.DirUp) << 0
		mask |= getBit(util.DirDown) << 1
		mask |= getBit(util.DirWest) << 2
		mask |= getBit(util.DirEast) << 3
	case util.DirWest, util.DirEast:
		mask |= getBit(util.DirUp) << 0
		mask |= getBit(util.DirDown) << 1
		mask |= getBit(util.DirNorth) << 2
		mask |= getBit(util.DirSouth) << 3
	}
	return mask
}

func (m *BlockMesher) getQuadCornerColors(coord util.DFCoord, face util.Directions, baseColor [4]uint8, data *mapdata.MapDataStore) (c1, c2, c3, c4 [4]uint8) {
	applyAO := func(c [4]uint8, ao float32) [4]uint8 {
		return [4]uint8{uint8(float32(c[0]) * ao), uint8(float32(c[1]) * ao), uint8(float32(c[2]) * ao), c[3]}
	}

	getAO := func(d1, d2, d3 util.Directions) float32 {
		occ1 := m.isSolidAO(coord, d1, data)
		occ2 := m.isSolidAO(coord, d2, data)
		occ3 := m.isSolidAO(coord, d3, data)
		if occ1 && occ2 {
			return 0.8
		}
		res := 1.0
		if occ1 {
			res -= 0.05
		}
		if occ2 {
			res -= 0.05
		}
		if occ3 {
			res -= 0.05
		}
		return float32(res)
	}

	switch face {
	case util.DirUp:
		c1 = applyAO(baseColor, getAO(util.DirNorth, util.DirWest, util.DirNorthWest))
		c2 = applyAO(baseColor, getAO(util.DirNorth, util.DirEast, util.DirNorthEast))
		c3 = applyAO(baseColor, getAO(util.DirSouth, util.DirEast, util.DirSouthEast))
		c4 = applyAO(baseColor, getAO(util.DirSouth, util.DirWest, util.DirSouthWest))
	default:
		return baseColor, baseColor, baseColor, baseColor
	}
	return
}

func (m *BlockMesher) shouldDrawFace(tile *mapdata.Tile, dir util.Directions) bool {
	neighbor := tile.GetNeighbor(dir)
	if neighbor == nil || neighbor.Hidden {
		return true
	}
	neighborShape := neighbor.Shape()
	if neighborShape == dfproto.ShapeNoShape {
		return true
	}

	if (tile.Shape() == dfproto.ShapeWall || tile.Shape() == dfproto.ShapeFortification) &&
		(neighborShape == dfproto.ShapeWall || neighborShape == dfproto.ShapeFortification) {
		if tile.Shape() == dfproto.ShapeWall && neighborShape == dfproto.ShapeWall {
			return false
		}
		if dir == util.DirUp || dir == util.DirDown {
			return false
		}
		return true
	}

	if dir == util.DirDown && tile.Shape() == dfproto.ShapeFloor && neighborShape == dfproto.ShapeWall {
		return false
	}
	return true
}

func (m *BlockMesher) isSolidAO(coord util.DFCoord, dir util.Directions, data *mapdata.MapDataStore) bool {
	neighborPos := coord.AddDir(dir)
	tile := data.GetTile(neighborPos)
	if tile == nil {
		return false
	}
	shape := tile.Shape()
	return shape == dfproto.ShapeWall || shape == dfproto.ShapeFortification
}

func (m *BlockMesher) isSolid(coord util.DFCoord, dir util.Directions, data *mapdata.MapDataStore) bool {
	neighborCoord := coord.Add(util.DirOffsets[dir])
	tile := data.GetTile(neighborCoord)
	if tile == nil {
		return false
	}
	shape := tile.Shape()
	return shape == dfproto.ShapeWall || shape == dfproto.ShapeRamp
}

func (m *BlockMesher) calculateAwayFromWallRotation(tile *mapdata.Tile) float32 {
	var vx, vz float32
	if n := tile.GetNeighbor(util.DirNorth); n != nil && (n.Shape() == dfproto.ShapeWall || n.Shape() == dfproto.ShapeFortification) {
		vz -= 1.0
	}
	if n := tile.GetNeighbor(util.DirSouth); n != nil && (n.Shape() == dfproto.ShapeWall || n.Shape() == dfproto.ShapeFortification) {
		vz += 1.0
	}
	if n := tile.GetNeighbor(util.DirWest); n != nil && (n.Shape() == dfproto.ShapeWall || n.Shape() == dfproto.ShapeFortification) {
		vx += 1.0
	}
	if n := tile.GetNeighbor(util.DirEast); n != nil && (n.Shape() == dfproto.ShapeWall || n.Shape() == dfproto.ShapeFortification) {
		vx -= 1.0
	}

	if vx == 0 && vz == 0 {
		return -1
	}
	return float32(math.Atan2(float64(vx), float64(vz))) * (180.0 / math.Pi)
}
