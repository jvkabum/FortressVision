package mapdata

import (
	"log"
	"sync"

	"FortressVision/shared/pkg/dfproto"
	"FortressVision/shared/util"

	"gorm.io/gorm"
)

// MapDataStore gerencia o armazenamento de tiles do mapa.
// Pode representar o mapa inteiro ou uma "fatia" (slice) local.
type MapDataStore struct {
	Mu sync.RWMutex

	// Chunks armazena os blocos do mapa (16x16x1)
	Chunks map[util.DFCoord]*Chunk

	// Tiletypes é um cache para consulta de propriedades (shape, material, etc)
	Tiletypes map[int32]*dfproto.Tiletype

	// MapSize é o tamanho total detectado do mapa (opcional)
	MapSize util.DFCoord

	// DB é a conexão com o banco SQLite (GORM)
	DB *gorm.DB
}

// ChangeType representa o tipo de mudança detectada em um bloco
type ChangeType int

const (
	NoChange         ChangeType = 0
	TerrainChange    ChangeType = 1
	VegetationChange ChangeType = 2
)

// Chunk representa um bloco 16x16x1 de tiles.
type Chunk struct {
	Origin  util.DFCoord
	Tiles   [16][16]*Tile
	Plants  []dfproto.PlantDetail // Cache de plantas (shrubs/saplings) para atualizações leves
	MTime   int64                 // Contador de modificações / versão
	IsDirty bool                  // Indica que o chunk foi alterado e precisa salvar
	IsEmpty bool                  // Indica que o bloco foi verificado e é conhecido como vazio (Ar/Céu)
}

// NewMapDataStore cria um novo repositório de dados do mapa.
func NewMapDataStore() *MapDataStore {
	return &MapDataStore{
		Chunks:    make(map[util.DFCoord]*Chunk),
		Tiletypes: make(map[int32]*dfproto.Tiletype),
	}
}

// MarkAsEmpty marca um chunk como conhecido e vazio (Ar).
// Isso evita que o servidor fique requisitando o mesmo céu repetidamente.
func (s *MapDataStore) MarkAsEmpty(origin util.DFCoord) {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	if _, exists := s.Chunks[origin]; !exists {
		s.Chunks[origin] = &Chunk{
			Origin:  origin,
			IsEmpty: true,
			MTime:   1, // Versão mínima
		}
	}
}

// GetTile retorna um tile em coordenadas globais. Retorna nil se não existir.
func (s *MapDataStore) GetTile(pos util.DFCoord) *Tile {
	s.Mu.RLock()
	defer s.Mu.RUnlock()

	blockPos := pos.BlockCoord()
	chunk, ok := s.Chunks[blockPos]
	if !ok {
		return nil
	}

	local := pos.LocalCoord()
	return chunk.Tiles[local.X][local.Y]
}

// GetOrCreateTile retorna um tile existente ou cria um novo se necessário.
func (s *MapDataStore) GetOrCreateTile(pos util.DFCoord) *Tile {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	blockPos := pos.BlockCoord()
	chunk, ok := s.Chunks[blockPos]
	if !ok {
		// Tenta carregamento do SQLite (Streaming)
		var err error
		chunk, err = s.LoadChunk(blockPos)
		if err != nil || chunk == nil {
			// Se não existe no banco, cria novo e marca como sujo (Write-Back)
			chunk = &Chunk{Origin: blockPos, IsDirty: true}
		}
		s.Chunks[blockPos] = chunk
	}

	local := pos.LocalCoord()
	tile := chunk.Tiles[local.X][local.Y]
	if tile == nil {
		tile = NewTile(s, pos)
		chunk.Tiles[local.X][local.Y] = tile
	}

	return tile
}

// StoreBlocks processa uma lista de blocos recebida do DFHack.
func (s *MapDataStore) StoreBlocks(list *dfproto.BlockList) {
	for _, block := range list.MapBlocks {
		s.StoreSingleBlock(&block)
	}
}

// StoreSingleBlock converte um bloco do Raw Proto e armazena/atualiza no store.
// Retorna o tipo de mudança detectada (NoChange, TerrainChange ou VegetationChange).
func (s *MapDataStore) StoreSingleBlock(block *dfproto.MapBlock) ChangeType {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	origin := util.NewDFCoord(block.MapX, block.MapY, block.MapZ).BlockCoord()
	chunk, ok := s.Chunks[origin]
	if !ok {
		chunk = &Chunk{Origin: origin}
		for y := 0; y < 16; y++ {
			for x := 0; x < 16; x++ {
				chunk.Tiles[x][y] = &Tile{}
			}
		}
		s.Chunks[origin] = chunk
		chunk.IsDirty = true // Primeiro carregamento sempre marca como dirty
		log.Printf("[Store] Chunk criado em RAM: %v (Origem DFHack: %d,%d,%d)", origin, block.MapX, block.MapY, block.MapZ)
	}

	// Flag para indicar se houve mudança real nos dados deste chunk
	chunkChanged := false
	vegChanged := false

	// Pré-processa os fluxos (Flows) para busca rápida por coordenada
	flowMap := make(map[util.DFCoord]util.DFCoord)
	for _, flow := range block.Flows {
		// Apenas guardamos a direção (Destino - Origem)
		if flow.Dest.X != flow.Pos.X || flow.Dest.Y != flow.Pos.Y || flow.Dest.Z != flow.Pos.Z {
			pos := util.NewDFCoord(flow.Pos.X, flow.Pos.Y, flow.Pos.Z)
			dir := util.NewDFCoord(flow.Dest.X-flow.Pos.X, flow.Dest.Y-flow.Pos.Y, flow.Dest.Z-flow.Pos.Z)

			// Normalizamos para 1 ou -1
			if dir.X > 0 {
				dir.X = 1
			} else if dir.X < 0 {
				dir.X = -1
			}
			if dir.Y > 0 {
				dir.Y = 1
			} else if dir.Y < 0 {
				dir.Y = -1
			}
			if dir.Z > 0 {
				dir.Z = 1
			} else if dir.Z < 0 {
				dir.Z = -1
			}

			flowMap[pos] = dir
		}
	}

	for yy := int32(0); yy < 16; yy++ {
		for xx := int32(0); xx < 16; xx++ {
			idx := xx + (yy * 16)
			// Nota: Chamamos GetOrCreateTileINTERNO sem lock, pois já temos o lock
			worldCoord := util.NewDFCoord(block.MapX+xx, block.MapY+yy, block.MapZ)

			// Lógica inline de GetOrCreateTile para evitar deadlock (recursão de mutex)
			blockPos := worldCoord.BlockCoord()
			chunk, ok := s.Chunks[blockPos]
			if !ok {
				// Tenta carregamento do SQLite (Streaming)
				var err error
				chunk, err = s.LoadChunk(blockPos)
				if err != nil || chunk == nil {
					// Se não existe no banco, cria novo e marca como sujo
					chunk = &Chunk{Origin: blockPos, IsDirty: true}
					chunkChanged = true
				}
				s.Chunks[blockPos] = chunk
			}

			local := worldCoord.LocalCoord()
			tile := chunk.Tiles[local.X][local.Y]
			if tile == nil {
				// NewTile precisa de 's' apenas para referência, não usa lock
				tile = NewTile(s, worldCoord)
				chunk.Tiles[local.X][local.Y] = tile
			}

			// Helper para verificar mudança
			checkChange := func(name string, current *int32, newVal int32) {
				if *current != newVal {
					*current = newVal
					chunkChanged = true
				}
			}
			checkChangeBool := func(name string, current *bool, newVal bool) {
				if *current != newVal {
					*current = newVal
					chunkChanged = true
				}
			}
			checkChangeMatPair := func(name string, current *dfproto.MatPair, newVal dfproto.MatPair) {
				if *current != newVal {
					*current = newVal
					chunkChanged = true
				}
			}
			checkChangeCoord := func(name string, current *util.DFCoord, newVal util.DFCoord) {
				if current.X != newVal.X || current.Y != newVal.Y || current.Z != newVal.Z {
					*current = newVal
					chunkChanged = true
				}
			}

			// Preenche dados básicos se presentes no bloco e marca se mudou
			if len(block.Tiles) > int(idx) {
				checkChange("TileType", &tile.TileType, block.Tiles[idx])
			}
			if len(block.Materials) > int(idx) {
				checkChangeMatPair("Material", &tile.Material, block.Materials[idx])
			}
			if len(block.BaseMaterials) > int(idx) {
				checkChangeMatPair("BaseMaterial", &tile.BaseMaterial, block.BaseMaterials[idx])
			}
			if len(block.LayerMaterials) > int(idx) {
				checkChangeMatPair("LayerMaterial", &tile.LayerMaterial, block.LayerMaterials[idx])
			}
			if len(block.VeinMaterials) > int(idx) {
				checkChangeMatPair("VeinMaterial", &tile.VeinMaterial, block.VeinMaterials[idx])
			}
			if len(block.Water) > int(idx) {
				checkChange("WaterLevel", &tile.WaterLevel, int32(block.Water[idx]))
			}
			if len(block.Magma) > int(idx) {
				checkChange("MagmaLevel", &tile.MagmaLevel, int32(block.Magma[idx]))
			}
			if len(block.Hidden) > int(idx) {
				checkChangeBool("Hidden", &tile.Hidden, block.Hidden[idx])
			}

			// Dados de árvores (Tronco e Galhos)
			if len(block.TreePercent) > int(idx) {
				newVal := uint8(block.TreePercent[idx])
				if tile.TrunkPercent != newVal {
					tile.TrunkPercent = newVal
					chunkChanged = true
				}
			}
			if len(block.TreeX) > int(idx) && len(block.TreeY) > int(idx) && len(block.TreeZ) > int(idx) {
				treePos := util.NewDFCoord(block.TreeX[idx], block.TreeY[idx], block.TreeZ[idx])
				checkChangeCoord("PositionOnTree", &tile.PositionOnTree, treePos)
			}

			// Procura por fluxo na posição global
			flowDir, hasFlow := flowMap[worldCoord]
			if hasFlow {
				checkChangeCoord("FlowVector", &tile.FlowVector, flowDir)
			} else {
				// Reseta se não houver mais fluxo
				checkChangeCoord("FlowVector", &tile.FlowVector, util.NewDFCoord(0, 0, 0))
			}
		}
	}

	// Coleta dados de vegetação (saplings e shrubs) para o cache do chunk
	var currentPlants []dfproto.PlantDetail
	chunkOrigin := util.NewDFCoord(block.MapX, block.MapY, block.MapZ).BlockCoord()
	chunk = s.Chunks[chunkOrigin]

	for yy := int32(0); yy < 16; yy++ {
		for xx := int32(0); xx < 16; xx++ {
			tile := chunk.Tiles[xx][yy]
			if tile != nil {
				shape := tile.Shape()
				if shape == dfproto.ShapeSapling || shape == dfproto.ShapeShrub {
					currentPlants = append(currentPlants, dfproto.PlantDetail{
						Pos:      dfproto.Coord{X: xx, Y: yy, Z: 0}, // Coordenada local
						Material: tile.Material,
					})
				}
			}
		}
	}

	// Compara com o cache anterior para detectar mudanças de vegetação
	if len(chunk.Plants) != len(currentPlants) {
		chunk.Plants = currentPlants
		vegChanged = true
	} else {
		for i := range currentPlants {
			if chunk.Plants[i].Pos != currentPlants[i].Pos ||
				chunk.Plants[i].Material != currentPlants[i].Material {
				chunk.Plants = currentPlants
				vegChanged = true
				break
			}
		}
	}

	// Só incrementa a versão do chunk se algo realmente mudou
	if chunkChanged || vegChanged {
		chunk.MTime++
		if !chunk.IsDirty {
			chunk.IsDirty = true
		}
	}

	if chunkChanged {
		return TerrainChange
	}
	if vegChanged {
		return VegetationChange
	}
	return NoChange
}

// StorePlants processa atualizações específicas de vegetação enviadas via rede.
func (s *MapDataStore) StorePlants(chunkX, chunkY, chunkZ int32, plants []dfproto.PlantDetail) {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	origin := util.NewDFCoord(chunkX, chunkY, chunkZ)
	chunk, ok := s.Chunks[origin]
	if !ok {
		return
	}

	chunk.Plants = plants
	chunk.MTime++
	chunk.IsDirty = true

	// Atualiza os materiais nos tiles para refletir o crescimento/mudança
	for _, p := range plants {
		if p.Pos.X >= 0 && p.Pos.X < 16 && p.Pos.Y >= 0 && p.Pos.Y < 16 {
			tile := chunk.Tiles[p.Pos.X][p.Pos.Y]
			if tile != nil {
				tile.Material = p.Material
			}
		}
	}
}

// UpdateTiletypes atualiza o cache de definições de tiles.
func (s *MapDataStore) UpdateTiletypes(list *dfproto.TiletypeList) {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	for i := range list.TiletypeList {
		tt := &list.TiletypeList[i]
		s.Tiletypes[tt.ID] = tt
	}
}

// Close fecha a conexão com o banco de dados SQLite.
func (s *MapDataStore) Close() {
	if s.DB != nil {
		sqlDB, _ := s.DB.DB()
		if sqlDB != nil {
			log.Println("[Persistence] Fechando banco de dados SQLite...")
			sqlDB.Close()
		}
	}
}

// Purge descarrega chunks distantes da RAM (streaming).
func (s *MapDataStore) Purge(center util.DFCoord, radius float32) {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	radiusSq := radius * radius
	for origin, chunk := range s.Chunks {
		dx := float32(origin.X - center.X)
		dy := float32(origin.Y - center.Y)
		dz := float32(origin.Z - center.Z)
		distSq := dx*dx + dy*dy + dz*dz

		if distSq > radiusSq {
			// WRITE-BACK: Antes de descarregar, salva se estiver sujo de forma ASSÍNCRONA
			if chunk.IsDirty {
				go s.SaveChunk(chunk) // Persiste em background para não travar o jogo
			}
			delete(s.Chunks, origin)
		}
	}
}

// HasData verifica se o banco de dados já possui algum chunk salvo para o mundo atual.
func (s *MapDataStore) HasData() bool {
	s.Mu.RLock()
	defer s.Mu.RUnlock()
	if s.DB == nil {
		return false
	}
	var count int64
	s.DB.Model(&ChunkModel{}).Count(&count)
	return count > 0
}

// QueueAllStoredChunks carrega todos os blocos do SQLite na tela inicializando o mapa 3D sem depender do socket.
func (s *MapDataStore) QueueAllStoredChunks(enqueueFunc func(origin util.DFCoord, mtime int64)) int {
	if s.DB == nil {
		return 0
	}

	// Retira apenas os metadados para não estourar a RAM
	var chunks []ChunkModel
	s.DB.Select("x", "y", "z", "m_time").Find(&chunks)

	s.Mu.Lock()
	for _, model := range chunks {
		origin := util.NewDFCoord(model.X, model.Y, model.Z)
		// Registra o chunk na memória como uma "casca" (shell)
		// Isso informa ao Scanner que o dado existe no SQL, evitando re-download.
		if _, exists := s.Chunks[origin]; !exists {
			s.Chunks[origin] = &Chunk{
				Origin: origin,
				MTime:  model.MTime,
			}
		}
	}
	s.Mu.Unlock()

	count := len(chunks)
	log.Printf("[Persistence] Enfileirando %d blocos carregados do SQLite local.", count)

	for _, model := range chunks {
		origin := util.NewDFCoord(model.X, model.Y, model.Z)
		enqueueFunc(origin, model.MTime)
	}

	return count
}
