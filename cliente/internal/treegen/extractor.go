package treegen

import (
	"FortressVision/shared/mapdata"
	"FortressVision/shared/util"
	"sync"
)

var (
	treeCache   = make(map[util.DFCoord]*TreeData)
	cacheMu     sync.RWMutex
)

// ClearCache limpa os dados temporários de extração
func ClearCache() {
	cacheMu.Lock()
	defer cacheMu.Unlock()
	treeCache = make(map[util.DFCoord]*TreeData)
}

// TreeData agrupa as informações de uma árvore específica para geração.
type TreeData struct {
	BaseCoord    util.DFCoord
	PlantID      uint32
	TrunkHeight  int
	CrownRadius  int
	TrunkTiles   []util.DFCoord
	BranchTiles  []util.DFCoord
	LeafTiles    []util.DFCoord
	TrunkPercent map[util.DFCoord]uint8
	DistFromBase map[util.DFCoord]int // Distância em blocos até a base
}

// Extractor extrai uma árvore inteira a partir de um tile de tronco.
func ExtractTree(base util.DFCoord, storage *mapdata.MapDataStore) *TreeData {
	cacheMu.RLock()
	if data, ok := treeCache[base]; ok {
		cacheMu.RUnlock()
		return data
	}
	cacheMu.RUnlock()

	tile := storage.GetTile(base)
	if tile == nil || (tile.Shape() != 138 && tile.Shape() != 139) { // TrunkBranch / Trunk
		return nil
	}

	data := &TreeData{
		BaseCoord:    base,
		PlantID:      uint32(tile.Material.MatIndex),
		TrunkPercent: make(map[util.DFCoord]uint8),
		DistFromBase: make(map[util.DFCoord]int),
	}

	// Busca tiles conectados num raio razoável (Dwarf Fortress costuma ter árvores de até ~15-20 blocos)
	// Para v1, faremos um scan local 11x11x20
	for dz := int32(0); dz < 20; dz++ {
		for dy := int32(-5); dy <= 5; dy++ {
			for dx := int32(-5); dx <= 5; dx++ {
				currPos := util.DFCoord{X: base.X + dx, Y: base.Y + dy, Z: base.Z + dz}
				t := storage.GetTile(currPos)
				if t == nil || uint32(t.Material.MatIndex) != data.PlantID {
					continue
				}

				shape := t.Shape()
				data.DistFromBase[currPos] = int(util.Abs(dx) + util.Abs(dy) + util.Abs(dz))

				switch shape {
				case 138, 139: // Trunk
					data.TrunkTiles = append(data.TrunkTiles, currPos)
					data.TrunkPercent[currPos] = t.TrunkPercent
				case 140, 141: // Branch
					data.BranchTiles = append(data.BranchTiles, currPos)
				case 142, 143, 137: // Twig / Leaves
					data.LeafTiles = append(data.LeafTiles, currPos)
				}
			}
		}
	}

	cacheMu.Lock()
	// Cachear todos os tiles de tronco desta árvore para evitar re-extração
	for _, t := range data.TrunkTiles {
		treeCache[t] = data
	}
	cacheMu.Unlock()

	return data
}
