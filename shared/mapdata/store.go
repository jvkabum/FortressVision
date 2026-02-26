package mapdata

import (
	"log"
	"math"
	"sync"

	"FortressVision/shared/pkg/dfproto"
	"FortressVision/shared/util"

	"gorm.io/gorm"
)

// MapDataStore gerencia o armazenamento de tiles do mapa.
// Pode representar o mapa inteiro ou uma "fatia" (slice) local.
type MapDataStore struct {
	Mu sync.RWMutex

	// dbMu serializa escritas no banco SQLite (impede "database is locked")
	dbMu sync.Mutex

	// Chunks armazena os blocos do mapa (16x16x1)
	Chunks map[util.DFCoord]*Chunk

	// Tiletypes é um cache para consulta de propriedades (shape, material, etc)
	Tiletypes map[int32]*dfproto.Tiletype

	// Entidades Dinâmicas
	Buildings      map[int32]*BuildingInstance
	Units          map[int32]*UnitInstance
	BuildingLookup map[util.DFCoord]int32 // Mapa de Coordenada -> ID da Construção

	// MapSize é o tamanho total detectado do mapa (opcional)
	MapSize util.DFCoord

	// DB é a conexão com o banco SQLite (GORM)
	DB *gorm.DB

	// FirstPerson indica se o sistema está em modo primeira pessoa (afeta desenho)
	FirstPerson bool

	// PosZ é o nível atual de foco (atualizado pelo servidor)
	PosZ int32
}

// RaycastHit armazena informações sobre uma colisão de raio.
type RaycastHit struct {
	TileCoord util.DFCoord
	Point     util.Vector3
	Distance  float32
}

// CollisionState representa o estado físico de um ponto no mapa.
type CollisionState int

const (
	CollisionNone   CollisionState = 0
	CollisionSolid  CollisionState = 1
	CollisionStairs CollisionState = 2
	CollisionWater  CollisionState = 3
)

// ChangeType representa o tipo de mudança detectada em um bloco
type ChangeType int

const (
	NoChange         ChangeType = 0
	TerrainChange    ChangeType = 1
	VegetationChange ChangeType = 2
)

// Chunk representa um bloco 16x16x1 de tiles.
type Chunk struct {
	Origin            util.DFCoord
	Tiles             [16][16]*Tile
	Plants            []dfproto.PlantDetail      // Cache de plantas (shrubs/saplings)
	Buildings         []dfproto.BuildingInstance // Construções no bloco
	Items             []dfproto.Item             // Itens soltos no bloco
	ConstructionItems []dfproto.MatPair          // Itens de construção (ex: paredes manuais)
	SpatterPile       []dfproto.SpatterPile      // Manchas e sujeiras (ID 25)
	Engravings        []dfproto.Engraving        // Gravuras e entalhes (ID 32)
	MTime             int64                      // Contador de modificações / versão
	IsDirty           bool                       // Indica que o chunk foi alterado e precisa salvar
	IsEmpty           bool                       // Indica que o bloco é ar/vazio
}

// NewMapDataStore cria um novo repositório de dados do mapa.
func NewMapDataStore() *MapDataStore {
	return &MapDataStore{
		Chunks:         make(map[util.DFCoord]*Chunk),
		Tiletypes:      make(map[int32]*dfproto.Tiletype),
		Buildings:      make(map[int32]*BuildingInstance),
		Units:          make(map[int32]*UnitInstance),
		BuildingLookup: make(map[util.DFCoord]int32),
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
			MTime:   1,    // Versão mínima
			IsDirty: true, // DEVE ser salvo no banco para não perder a informação do vazio no Purge
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

// GetChunk retorna um chunk de forma segura (thread-safe).
func (s *MapDataStore) GetChunk(origin util.DFCoord) (*Chunk, bool) {
	s.Mu.RLock()
	defer s.Mu.RUnlock()
	c, ok := s.Chunks[origin]
	return c, ok
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

	// O DFHack (via protobuf) agora devolve MapX e MapY como índices de BLOCOS inteiros (0 a TotalBlocks).
	// Precisamos convertê-los de volta em coordenadas absolutas de TILEs mundo (*16) antes do loop.
	baseTileX := block.MapX * 16
	baseTileY := block.MapY * 16

	for yy := int32(0); yy < 16; yy++ {
		for xx := int32(0); xx < 16; xx++ {
			idx := xx + (yy * 16)
			// Nota: Chamamos GetOrCreateTileINTERNO sem lock, pois já temos o lock
			worldCoord := util.NewDFCoord(baseTileX+xx, baseTileY+yy, block.MapZ)

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
			if len(block.Light) > int(idx) {
				checkChangeBool("Light", &tile.Light, block.Light[idx])
			}
			if len(block.Subterranean) > int(idx) {
				checkChangeBool("Subterranean", &tile.Subterranean, block.Subterranean[idx])
			}
			if len(block.Outside) > int(idx) {
				checkChangeBool("Outside", &tile.Outside, block.Outside[idx])
			}
			if len(block.Aquifer) > int(idx) {
				checkChangeBool("Aquifer", &tile.Aquifer, block.Aquifer[idx])
			}
			if len(block.WaterStagnant) > int(idx) {
				checkChangeBool("WaterStagnant", &tile.WaterStagnant, block.WaterStagnant[idx])
			}
			if len(block.WaterSalt) > int(idx) {
				checkChangeBool("WaterSalt", &tile.WaterSalt, block.WaterSalt[idx])
			}
			if len(block.ConstructionItems) > int(idx) {
				checkChangeMatPair("ConstructionItem", &tile.ConstructionItem, block.ConstructionItems[idx])
			}
			if len(block.TileDigDesignation) > int(idx) {
				newDig := block.TileDigDesignation[idx]
				if tile.DigDesignation != newDig {
					tile.DigDesignation = newDig
					chunkChanged = true
				}
			}
			if len(block.DigDesignationMarker) > int(idx) {
				checkChangeBool("DigMarker", &tile.DigMarker, block.DigDesignationMarker[idx])
			}
			if len(block.DigDesignationAuto) > int(idx) {
				checkChangeBool("DigAuto", &tile.DigAuto, block.DigDesignationAuto[idx])
			}
			if len(block.GrassPercent) > int(idx) {
				checkChange("GrassPercent", &tile.GrassPercent, block.GrassPercent[idx])
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

	// Sincroniza Entidades (Construções e Itens)
	if len(block.Buildings) > 0 || len(chunk.Buildings) > 0 {
		chunk.Buildings = block.Buildings
		chunkChanged = true
	}
	if len(block.Items) > 0 || len(chunk.Items) > 0 {
		chunk.Items = block.Items
		chunkChanged = true
	}
	if len(block.ConstructionItems) > 0 || len(chunk.ConstructionItems) > 0 {
		chunk.ConstructionItems = block.ConstructionItems
		chunkChanged = true
	}
	if len(block.SpatterPile) > 0 || len(chunk.SpatterPile) > 0 {
		chunk.SpatterPile = block.SpatterPile
		chunkChanged = true
	}
	if len(block.Engravings) > 0 || len(chunk.Engravings) > 0 {
		chunk.Engravings = block.Engravings
		chunkChanged = true
	}

	// Coleta dados de vegetação (saplings e shrubs)
	var currentPlants []dfproto.PlantDetail
	if len(block.Plants) > 0 {
		currentPlants = block.Plants
	} else {
		// Fallback para detecção por shape (compatibilidade ou blocos incompletos)
		for yy := int32(0); yy < 16; yy++ {
			for xx := int32(0); xx < 16; xx++ {
				tile := chunk.Tiles[xx][yy]
				if tile != nil {
					shape := tile.Shape()
					if shape == dfproto.ShapeSapling || shape == dfproto.ShapeShrub {
						currentPlants = append(currentPlants, dfproto.PlantDetail{
							Pos:      dfproto.Coord{X: xx, Y: yy, Z: 0},
							Material: tile.Material,
						})
					}
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
				// Salva em background com dbMu para serializar escritas no banco
				chunkCopy := chunk
				go func() {
					s.dbMu.Lock()
					s.SaveChunk(chunkCopy)
					s.dbMu.Unlock()
				}()
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

// InMapBounds verifica se a coordenada está dentro dos limites do mapa.
func (s *MapDataStore) InMapBounds(coord util.DFCoord) bool {
	return coord.X >= 0 && coord.X < s.MapSize.X &&
		coord.Y >= 0 && coord.Y < s.MapSize.Y &&
		coord.Z >= 0 && coord.Z < s.MapSize.Z
}

// HitsMapCube verifica se o raio atinge o cubo delimitador do mapa.
func (s *MapDataStore) HitsMapCube(ray util.Ray) bool {
	if s.MapSize.X == 0 {
		return false
	}

	lowerLimits := util.DFToWorldBottomCorner(util.DFCoord{0, 0, 0})
	upperLimits := util.DFToWorldBottomCorner(util.DFCoord{
		s.MapSize.X - 1,
		s.MapSize.Y - 1,
		s.MapSize.Z - 1,
	})
	upperLimits.X += util.GameScale
	upperLimits.Y += util.GameScale
	upperLimits.Z -= util.GameScale // Z é invertido no nosso sistema

	// AABB intersection simples
	tmin := float32(-math.MaxFloat32)
	tmax := float32(math.MaxFloat32)

	// X
	if ray.Direction.X != 0 {
		tx1 := (lowerLimits.X - ray.Origin.X) / ray.Direction.X
		tx2 := (upperLimits.X - ray.Origin.X) / ray.Direction.X
		tmin = float32(math.Max(float64(tmin), math.Min(float64(tx1), float64(tx2))))
		tmax = float32(math.Min(float64(tmax), math.Max(float64(tx1), float64(tx2))))
	}
	// Y (Z no DF)
	if ray.Direction.Y != 0 {
		ty1 := (lowerLimits.Y - ray.Origin.Y) / ray.Direction.Y
		ty2 := (upperLimits.Y - ray.Origin.Y) / ray.Direction.Y
		tmin = float32(math.Max(float64(tmin), math.Min(float64(ty1), float64(ty2))))
		tmax = float32(math.Min(float64(tmax), math.Max(float64(ty1), float64(ty2))))
	}
	// Z (Y no DF)
	if ray.Direction.Z != 0 {
		tz1 := (lowerLimits.Z - ray.Origin.Z) / ray.Direction.Z
		tz2 := (upperLimits.Z - ray.Origin.Z) / ray.Direction.Z
		tmin = float32(math.Max(float64(tmin), math.Min(float64(tz1), float64(tz2))))
		tmax = float32(math.Min(float64(tmax), math.Max(float64(tz1), float64(tz2))))
	}

	return tmin <= tmax && tmax > 0
}

// Raycast realiza o rastreamento de raio através dos tiles do mapa.
func (s *MapDataStore) Raycast(ray util.Ray, maxDistance float32) (*RaycastHit, bool) {
	if !s.HitsMapCube(ray) {
		return nil, false
	}

	const maxChecks = 1000
	currentCoord := util.WorldToDFCoord(ray.Origin)
	lastHit := ray.Origin
	haveHitMap := false

	var xWallOffset, yWallOffset, zWallOffset float32
	var xHitInc, yHitInc, zHitInc util.DFCoord

	if ray.Direction.X > 0 {
		xWallOffset = util.GameScale
		xHitInc = util.DFCoord{1, 0, 0}
	} else {
		xWallOffset = 0
		xHitInc = util.DFCoord{-1, 0, 0}
	}

	if ray.Direction.Z > 0 { // Unity Z is -DF Y
		zWallOffset = -util.GameScale // Direction is positive Z (decreasing DF Y)
		zHitInc = util.DFCoord{0, -1, 0}
	} else {
		zWallOffset = 0
		zHitInc = util.DFCoord{0, 1, 0}
	}

	if ray.Direction.Y > 0 { // Unity Y is DF Z
		yWallOffset = util.GameScale
		yHitInc = util.DFCoord{0, 0, 1}
	} else {
		yWallOffset = 0
		yHitInc = util.DFCoord{0, 0, -1}
	}

	for i := 0; i < maxChecks; i++ {
		corner := util.DFToWorldBottomCorner(currentCoord)

		if !s.InMapBounds(currentCoord) || (s.PosZ > 0 && currentCoord.Z >= s.PosZ) {
			if haveHitMap {
				return nil, false
			}
		} else {
			haveHitMap = true
			tile := s.GetTile(currentCoord)
			if tile != nil {
				shape := tile.Shape()
				switch shape {
				case dfproto.ShapeWall, dfproto.ShapeTreeShape, dfproto.ShapeTwig, dfproto.ShapeTrunkBranch:
					return &RaycastHit{TileCoord: currentCoord, Point: lastHit}, true

				case dfproto.ShapeFloor, dfproto.ShapeBoulder, dfproto.ShapePebbles, dfproto.ShapeSapling, dfproto.ShapeShrub:
					floorY := corner.Y + util.FloorHeight
					if util.Between(corner.Y, lastHit.Y, floorY) {
						return &RaycastHit{TileCoord: currentCoord, Point: lastHit}, true
					}
					// Verificação de entrada no piso
					if ray.Direction.Y != 0 {
						toFloorMult := (floorY - ray.Origin.Y) / ray.Direction.Y
						intersect := util.Vector3{
							X: ray.Origin.X + ray.Direction.X*toFloorMult,
							Y: floorY,
							Z: ray.Origin.Z + ray.Direction.Z*toFloorMult,
						}
						// Verificamos se o ponto de intersecção está dentro do tile (X e Z)
						// Z é invertido, então logicamente corner.Z é o limite leste/oeste?
						// Não, DFToWorldPos inverte Y para Z.
						if util.Between(corner.X, intersect.X, corner.X+util.GameScale) &&
							util.Between(corner.Z-util.GameScale, intersect.Z, corner.Z) {
							return &RaycastHit{TileCoord: currentCoord, Point: intersect}, true
						}
					}

				case dfproto.ShapeRamp:
					rampTopY := corner.Y + util.GameScale/2 + util.FloorHeight
					if util.Between(corner.Y, lastHit.Y, rampTopY) {
						return &RaycastHit{TileCoord: currentCoord, Point: lastHit}, true
					}
					if ray.Direction.Y != 0 {
						toRampMult := (rampTopY - ray.Origin.Y) / ray.Direction.Y
						intersect := util.Vector3{
							X: ray.Origin.X + ray.Direction.X*toRampMult,
							Y: rampTopY,
							Z: ray.Origin.Z + ray.Direction.Z*toRampMult,
						}
						if util.Between(corner.X, intersect.X, corner.X+util.GameScale) &&
							util.Between(corner.Z-util.GameScale, intersect.Z, corner.Z) {
							return &RaycastHit{TileCoord: currentCoord, Point: intersect}, true
						}
					}
				}
			}
		}

		// Avançar para o próximo tile baseado na intersecção com as paredes
		minDist := float32(math.MaxFloat32)
		var nextCoord util.DFCoord
		var nextHit util.Vector3
		found := false

		// X Wall
		if ray.Direction.X != 0 {
			mult := (corner.X + xWallOffset - ray.Origin.X) / ray.Direction.X
			if mult > 0 {
				intersect := util.Vector3{
					X: ray.Origin.X + ray.Direction.X*mult,
					Y: ray.Origin.Y + ray.Direction.Y*mult,
					Z: ray.Origin.Z + ray.Direction.Z*mult,
				}
				if util.Between(corner.Z-util.GameScale, intersect.Z, corner.Z) &&
					util.Between(corner.Y, intersect.Y, corner.Y+util.GameScale) {
					minDist = mult
					nextCoord = currentCoord.Add(xHitInc)
					nextHit = intersect
					found = true
				}
			}
		}

		// Z Wall (Unity Z / DF Y)
		if ray.Direction.Z != 0 {
			mult := (corner.Z + zWallOffset - ray.Origin.Z) / ray.Direction.Z
			if mult > 0 && mult < minDist {
				intersect := util.Vector3{
					X: ray.Origin.X + ray.Direction.X*mult,
					Y: ray.Origin.Y + ray.Direction.Y*mult,
					Z: ray.Origin.Z + ray.Direction.Z*mult,
				}
				if util.Between(corner.X, intersect.X, corner.X+util.GameScale) &&
					util.Between(corner.Y, intersect.Y, corner.Y+util.GameScale) {
					minDist = mult
					nextCoord = currentCoord.Add(zHitInc)
					nextHit = intersect
					found = true
				}
			}
		}

		// Y Wall (Unity Y / DF Z)
		if ray.Direction.Y != 0 {
			mult := (corner.Y + yWallOffset - ray.Origin.Y) / ray.Direction.Y
			if mult > 0 && mult < minDist {
				intersect := util.Vector3{
					X: ray.Origin.X + ray.Direction.X*mult,
					Y: ray.Origin.Y + ray.Direction.Y*mult,
					Z: ray.Origin.Z + ray.Direction.Z*mult,
				}
				if util.Between(corner.X, intersect.X, corner.X+util.GameScale) &&
					util.Between(corner.Z-util.GameScale, intersect.Z, corner.Z) {
					minDist = mult
					nextCoord = currentCoord.Add(yHitInc)
					nextHit = intersect
					found = true
				}
			}
		}

		if !found || minDist > maxDistance {
			break
		}
		currentCoord = nextCoord
		lastHit = nextHit
	}

	return nil, false
}

// CheckCollision verifica o estado físico em uma posição 3D.
func (s *MapDataStore) CheckCollision(pos util.Vector3) CollisionState {
	dfPos := util.WorldToDFCoord(pos)
	tile := s.GetTile(dfPos)
	if tile == nil {
		return CollisionNone
	}

	corner := util.DFToWorldPos(dfPos)
	// localY varia de 0 a 1.0 (GameScale) de acordo com o Z do DF
	localY := pos.Y - corner.Y

	shape := tile.Shape()
	state := CollisionNone

	switch shape {
	case dfproto.ShapeWall, dfproto.ShapeFortification, dfproto.ShapeTreeShape:
		state = CollisionSolid
	case dfproto.ShapeFloor, dfproto.ShapeBoulder, dfproto.ShapePebbles, dfproto.ShapeSapling, dfproto.ShapeShrub, dfproto.ShapeBranch, dfproto.ShapeTrunkBranch:
		if localY < util.FloorHeight {
			state = CollisionSolid
		}
	case dfproto.ShapeStairUp:
		if localY < util.FloorHeight {
			state = CollisionSolid
		} else {
			state = CollisionStairs
		}
	case dfproto.ShapeStairDown:
		if localY < util.FloorHeight {
			state = CollisionStairs
		}
	case dfproto.ShapeStairUpDown:
		state = CollisionStairs
	case dfproto.ShapeRamp:
		if localY < util.FloorHeight {
			state = CollisionSolid
		}
	}

	// Adicionando verificação de líquidos
	if localY < (float32(tile.WaterLevel)/7.0)*util.GameScale {
		state = CollisionWater
	}
	if localY < (float32(tile.MagmaLevel)/7.0)*util.GameScale {
		state = CollisionWater
	}

	return state
}

// AddBuilding adiciona ou atualiza uma construção no store e indexa seus tiles.
func (s *MapDataStore) AddBuilding(b *BuildingInstance) {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	s.Buildings[b.Index] = b

	// Indexar todos os tiles ocupados pela construção
	for zz := b.MinPos.Z; zz <= b.MaxPos.Z; zz++ {
		for yy := b.MinPos.Y; yy <= b.MaxPos.Y; yy++ {
			for xx := b.MinPos.X; xx <= b.MaxPos.X; xx++ {
				pos := util.DFCoord{X: xx, Y: yy, Z: zz}
				s.BuildingLookup[pos] = b.Index
			}
		}
	}
}

// GetBuildingAt retorna a construção em uma coordenada específica.
func (s *MapDataStore) GetBuildingAt(pos util.DFCoord) *BuildingInstance {
	s.Mu.RLock()
	defer s.Mu.RUnlock()

	id, ok := s.BuildingLookup[pos]
	if !ok {
		return nil
	}
	return s.Buildings[id]
}

// UpdateUnit adiciona ou atualiza uma unidade no store.
func (s *MapDataStore) UpdateUnit(u *UnitInstance) {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	s.Units[u.ID] = u
}

// ClearEntities remove todas as entidades (útil ao mudar de mapa).
func (s *MapDataStore) ClearEntities() {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	s.Buildings = make(map[int32]*BuildingInstance)
	s.Units = make(map[int32]*UnitInstance)
	s.BuildingLookup = make(map[util.DFCoord]int32)
}
