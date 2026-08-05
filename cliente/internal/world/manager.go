package world

import (
	"bytes"
	"encoding/gob"
	"log"
	"sync"
	"time"

	"FortressVision/shared/mapdata"
	"FortressVision/shared/pkg/dfproto"
	"FortressVision/shared/proto/fvnet"
	"FortressVision/shared/util"
	"google.golang.org/protobuf/proto"
)

// Manager é o guardião dos dados do jogo (Chunks, Materiais, Entidades).
// Ele consome as informações cruas da rede e organiza no MapDataStore,
// disponibilizando avisos estruturados para outros sistemas (como o Mesher).
type Manager struct {
	Store *mapdata.MapDataStore

	regionMu     sync.RWMutex
	activeRegion *regionWindow

	// Eventos
	OnMapChunkUpdated func(chunk *mapdata.Chunk)
	OnMapChunkRemoved func(origin util.DFCoord)
	OnUnitsUpdated    func(units []mapdata.UnitInstance)
	OnWorldStatus     func(status *fvnet.WorldStatus)
	OnStatusMsg       func(msg string, dfConnected bool)
}

type regionWindow struct {
	center     util.DFCoord
	radius     int32
	levelsDown int32
	levelsUp   int32
}

func (r regionWindow) contains(origin util.DFCoord) bool {
	minX := r.center.X - r.radius
	maxX := r.center.X + r.radius
	minY := r.center.Y - r.radius
	maxY := r.center.Y + r.radius
	if origin.X > maxX || origin.X+util.BlockSize-1 < minX {
		return false
	}
	if origin.Y > maxY || origin.Y+util.BlockSize-1 < minY {
		return false
	}
	return origin.Z >= r.center.Z-r.levelsDown && origin.Z <= r.center.Z+r.levelsUp
}

// NewManager cria uma nova gestão de mundo com armazenamento vazio.
func NewManager() *Manager {
	return &Manager{
		Store: mapdata.NewMapDataStore(),
	}
}

// SetActiveRegion atualiza a janela solicitada pelo cliente e remove do
// store/renderizador os chunks que ficaram definitivamente fora dela. Sem
// essa poda, cada deslocamento acumulava malhas e entidades antigas e o
// cliente podia piscar ou encerrar ao caminhar rapidamente pelo mapa.
func (m *Manager) SetActiveRegion(center util.DFCoord, radius, levelsDown, levelsUp int32) {
	if radius < 0 {
		radius = 0
	}
	if levelsDown < 0 {
		levelsDown = 0
	}
	if levelsDown > 32 {
		levelsDown = 32
	}
	if levelsUp < 0 {
		levelsUp = 0
	}
	if levelsUp > 32 {
		levelsUp = 32
	}
	region := regionWindow{
		center:     center,
		radius:     radius,
		levelsDown: levelsDown,
		levelsUp:   levelsUp,
	}

	m.regionMu.Lock()
	m.activeRegion = &region
	m.regionMu.Unlock()

	removed := make([]util.DFCoord, 0)
	m.Store.Mu.Lock()
	for origin := range m.Store.Chunks {
		if !region.contains(origin) {
			delete(m.Store.Chunks, origin)
			removed = append(removed, origin)
		}
	}
	m.Store.Mu.Unlock()

	if m.OnMapChunkRemoved != nil {
		for _, origin := range removed {
			m.OnMapChunkRemoved(origin)
		}
	}
}

func (m *Manager) acceptsOrigin(origin util.DFCoord) bool {
	m.regionMu.RLock()
	region := m.activeRegion
	if region == nil {
		m.regionMu.RUnlock()
		return true
	}
	accepted := region.contains(origin)
	m.regionMu.RUnlock()
	return accepted
}

// HandleEnvelope processa os envelopes (mensagens) recebidos da rede.
func (m *Manager) HandleEnvelope(env *fvnet.Envelope) {
	switch env.Type {
	case fvnet.Envelope_SERVER_STATUS:
		var status fvnet.ServerStatus
		if err := proto.Unmarshal(env.Payload, &status); err == nil {
			log.Printf("[World] 🟢 Servidor Status: %s (DF: %v)", status.Message, status.DfConnected)
			if m.OnStatusMsg != nil {
				m.OnStatusMsg(status.Message, status.DfConnected)
			}
		}

	case fvnet.Envelope_WORLD_STATUS:
		var worldStatus fvnet.WorldStatus
		if err := proto.Unmarshal(env.Payload, &worldStatus); err == nil {
			log.Printf("[World] 🌍 Relatório do Mundo: %s, Ano %d (Foco: %d,%d, Z:%d)",
				worldStatus.WorldName, worldStatus.Year, worldStatus.ViewX, worldStatus.ViewY, worldStatus.ViewZ)
			if m.OnWorldStatus != nil {
				m.OnWorldStatus(&worldStatus)
			}
		}

	case fvnet.Envelope_TILETYPE_LIST:
		var list dfproto.TiletypeList
		if err := list.Unmarshal(env.Payload); err == nil {
			log.Printf("[World] 🧱 Recebidos %d Tiletypes do servidor", len(list.TiletypeList))
			if len(list.TiletypeList) > 0 {
				tt := list.TiletypeList[0]
				log.Printf("[World-Diag] 🧱 Exemplo Tiletype[0]: ID=%d, Nome=%s, Shape=%v", tt.ID, tt.Name, tt.Shape)
			}
			m.Store.UpdateTiletypes(&list)
		}

	case fvnet.Envelope_MATERIAL_LIST:
		var list dfproto.MaterialList
		if err := list.Unmarshal(env.Payload); err == nil {
			log.Printf("[World] 🎨 Recebidos %d Materiais do servidor", len(list.MaterialList))
			m.Store.UpdateMaterials(&list)
		}

	case fvnet.Envelope_MAP_CHUNK:
		var chunkMsg fvnet.MapChunkMessage
		if err := proto.Unmarshal(env.Payload, &chunkMsg); err == nil {
			log.Printf("[World] 🧱 Envelope MAP_CHUNK recebido p/ %d,%d,%d (VoxelData: %d bytes)", chunkMsg.ChunkX, chunkMsg.ChunkY, chunkMsg.ChunkZ, len(chunkMsg.VoxelData))
			m.processChunk(&chunkMsg)
		} else {
			log.Printf("[World] ❌ Erro ao desmarshallar MAP_CHUNK: %v", err)
		}

	case fvnet.Envelope_CREATURE_UPDATE:
		var creatures fvnet.CreatureUpdateMessage
		if err := creatures.Unmarshal(env.Payload); err != nil {
			log.Printf("[World] Erro ao desserializar unidades: %v", err)
			break
		}
		m.Store.ReplaceUnits(creatures.Units)
		if m.OnUnitsUpdated != nil {
			m.OnUnitsUpdated(creatures.Units)
		}

	case fvnet.Envelope_VEGETATION_UPDATE:
		var vegMsg fvnet.VegetationUpdateMessage
		if err := vegMsg.Unmarshal(env.Payload); err == nil {
			m.processVegetation(&vegMsg)
		}
	}
}

// processChunk extrai do Envelope o bloco codificado com GOB e armazena na memória (Store).
func (m *Manager) processChunk(msg *fvnet.MapChunkMessage) {
	origin := util.DFCoord{X: msg.ChunkX, Y: msg.ChunkY, Z: msg.ChunkZ}
	if !m.acceptsOrigin(origin) {
		// Um stream antigo pode concluir depois que a cÃ¢mera jÃ¡ mudou de
		// regiÃ£o. Ignorar essa mensagem impede que geometria obsoleta volte ao
		// renderer e cresÃ§a indefinidamente na memÃ³ria.
		return
	}

	// Chunk de "Ar" (vazio)
	if msg.VoxelData == nil {
		m.Store.Mu.Lock()
		delete(m.Store.Chunks, origin)
		m.Store.Mu.Unlock()
		if m.OnMapChunkUpdated != nil {
			m.OnMapChunkUpdated(nil) // Sinaliza remoção/ar
		}
		if m.OnMapChunkRemoved != nil {
			m.OnMapChunkRemoved(origin)
		}
		return
	}

	// Decodificador de voxel-grid serializado do server (16x16)
	var snapshot mapdata.ChunkSnapshot
	dec := gob.NewDecoder(bytes.NewReader(msg.VoxelData))
	if err := dec.Decode(&snapshot); err != nil {
		log.Printf("[World] ❌ Erro ao decodificar dados do chunk em %v: %v", origin, err)
		return
	}
	var tiles [16][16]*mapdata.Tile
	for x := 0; x < 16; x++ {
		for y := 0; y < 16; y++ {
			if snapshot.TilePresent[x][y] {
				tile := snapshot.Tiles[x][y]
				tiles[x][y] = &tile
			}
		}
	}

	m.Store.Mu.Lock()
	chunk := &mapdata.Chunk{
		Origin:            origin,
		Tiles:             tiles,
		Plants:            snapshot.Plants,
		Buildings:         snapshot.Buildings,
		Items:             snapshot.Items,
		ConstructionItems: snapshot.ConstructionItems,
		Flows:             snapshot.Flows,
		SpatterPile:       snapshot.SpatterPile,
		Engravings:        snapshot.Engravings,
		OceanWaves:        snapshot.OceanWaves,
		MTime:             time.Now().UnixNano(),
	}

	// Reconectar as referências recursivas dos tiles ao seu armazenador de mundo
	for x := 0; x < 16; x++ {
		for y := 0; y < 16; y++ {
			if t := chunk.Tiles[x][y]; t != nil {
				t.SetStore(m.Store)
				t.Position = util.NewDFCoord(origin.X+int32(x), origin.Y+int32(y), origin.Z)
			}
		}
	}

	m.Store.Chunks[origin] = chunk
	m.Store.Mu.Unlock()

	affectedChunks := m.Store.RecalculateRampTypes(origin)

	log.Printf("[World] ✅ Chunk %v processado com sucesso (%d tiles)", origin, 16*16)

	// Avisizar interessados (como o render/mesher) que este terreno atualizou.
	if m.OnMapChunkUpdated != nil {
		m.OnMapChunkUpdated(chunk)
		for _, affectedOrigin := range affectedChunks {
			if affectedOrigin == origin {
				continue
			}
			if affected, ok := m.Store.GetChunk(affectedOrigin); ok {
				m.OnMapChunkUpdated(affected)
			}
		}
	}
}

// processVegetation propaga mudanças apenas nas vegetações (Shrubs, Trees) para o Store.
func (m *Manager) processVegetation(msg *fvnet.VegetationUpdateMessage) {
	origin := util.DFCoord{X: msg.ChunkX, Y: msg.ChunkY, Z: msg.ChunkZ}
	if !m.acceptsOrigin(origin) {
		return
	}

	var plants []dfproto.PlantDetail
	for _, p := range msg.Plants {
		plants = append(plants, dfproto.PlantDetail{
			Pos:      dfproto.Coord{X: p.X, Y: p.Y, Z: origin.Z},
			Material: dfproto.MatPair{MatType: p.MatType, MatIndex: p.MatIndex},
		})
	}

	m.Store.StorePlants(msg.ChunkX, msg.ChunkY, msg.ChunkZ, plants)

	// A vegetação crescer é uma atualização de chunk. Notificar com o chunk atualizado.
	if m.OnMapChunkUpdated != nil {
		chunk, _ := m.Store.GetChunk(origin)
		m.OnMapChunkUpdated(chunk)
	}
}

// RequestRegion pede ativamente à "Estrada" (cliente WebSocket) um raio contendo terreno a partir do foco central.
func (m *Manager) RequestRegion(sendFunc func(msgType fvnet.Envelope_Type, msg proto.Message), center util.DFCoord, radius, levelsDown, levelsUp int32) {
	if levelsDown < 0 {
		levelsDown = 0
	}
	if levelsUp < 0 {
		levelsUp = 0
	}
	m.SetActiveRegion(center, radius, levelsDown, levelsUp)
	req := &fvnet.ClientRequestRegion{
		CenterX:    center.X,
		CenterY:    center.Y,
		CenterZ:    center.Z,
		Radius:     radius,
		LevelsDown: levelsDown,
		LevelsUp:   levelsUp,
	}
	sendFunc(fvnet.Envelope_CLIENT_REQUEST_REGION, req)
}
