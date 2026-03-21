package meshing

import (
	"FortressVision/shared/mapdata"
	"FortressVision/shared/util"
)

// findParentTrunk localiza o tronco que origina este galho
func (m *BlockMesher) findParentTrunk(coord util.DFCoord, data *mapdata.MapDataStore) *mapdata.Tile {
	// Direções possíveis para procurar o tronco pai
	directions := []util.Directions{
		util.DirDown,
		util.DirUp,
		util.DirNorth,
		util.DirSouth,
		util.DirWest,
		util.DirEast,
	}
	
	tile := data.GetTile(coord)
	if tile != nil && (tile.PositionOnTree.X != 0 || tile.PositionOnTree.Y != 0) {
		// Galho tem direção definida, procurar na direção oposta
		opposite := util.DFCoord{
			X: coord.X - tile.PositionOnTree.X,
			Y: coord.Y - tile.PositionOnTree.Y,
			Z: coord.Z - tile.PositionOnTree.Z,
		}
		
		if parent := data.GetTile(opposite); parent != nil {
			if parent.Shape() == 138 || parent.Shape() == 139 { // Trunk
				return parent
			}
		}
	}
	
	// Fallback: procurar nas direções principais
	for _, dir := range directions {
		neighbor := coord.AddDir(dir)
		t := data.GetTile(neighbor)
		if t != nil && (t.Shape() == 138 || t.Shape() == 139) { // Trunk
			return t
		}
	}
	
	return nil
}

// findTrunkTree retorna todos os tiles de tronco da árvore que contém este galho (BFS)
func (m *BlockMesher) findTrunkTree(coord util.DFCoord, data *mapdata.MapDataStore) []util.DFCoord {
	trunks := make([]util.DFCoord, 0)
	visited := make(map[util.DFCoord]bool)
	queue := []util.DFCoord{coord}
	
	tileAtOrig := data.GetTile(coord)
	if tileAtOrig == nil { return nil }

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		
		if visited[current] { continue }
		visited[current] = true
		
		tile := data.GetTile(current)
		if tile == nil { continue }
		
		if tile.Shape() == 138 || tile.Shape() == 139 {
			trunks = append(trunks, current)
		}
		
		// Limitar busca para não explodir
		if len(visited) > 100 { break }

		for _, offset := range util.DirOffsets {
			neighbor := current.Add(offset)
			if visited[neighbor] { continue }
			
			t := data.GetTile(neighbor)
			if t != nil && t.Material.MatIndex == tileAtOrig.Material.MatIndex {
				queue = append(queue, neighbor)
			}
		}
	}
	
	return trunks
}
