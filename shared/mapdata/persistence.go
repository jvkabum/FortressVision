package mapdata

import (
	"FortressVision/shared/pkg/dfproto"
	"FortressVision/shared/util"
	"bytes"
	"encoding/gob"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// ChunkModel representa o esquema do banco de dados para um chunk
type ChunkModel struct {
	ID        string    `gorm:"primaryKey"` // Coordenada formatada "X_Y_Z"
	X, Y, Z   int32     `gorm:"index:idx_pos"`
	Data      []byte    // Dados do chunk serializados em GOB
	MTime     int64     // Versão/Timestamp
	IsEmpty   bool      // Indica se o chunk é céu/ar puro
	UpdatedAt time.Time // Para controle interno do GORM
}

// WorldMetadata armazena informações globais do mundo no banco
type WorldMetadata struct {
	Key   string `gorm:"primaryKey"`
	Value string
}

// MaterialModel armazena a cor de um material específico persistido
type MaterialModel struct {
	MatType  int32 `gorm:"primaryKey;autoIncrement:false"`
	MatIndex int32 `gorm:"primaryKey;autoIncrement:false"`
	R, G, B  uint8
}

const CurrentFormatVersion = 4

// chunkData é o container para serialização completa de um bloco
type chunkData struct {
	Tiles             [16][16]*Tile
	Plants            []dfproto.PlantDetail
	Buildings         []dfproto.BuildingInstance
	Items             []dfproto.Item
	ConstructionItems []dfproto.MatPair
	SpatterPile       []dfproto.SpatterPile
	Engravings        []dfproto.Engraving
}

// OpenInitialize abre (ou cria) o banco de dados SQLite para o mundo e roda migrações.
func (s *MapDataStore) OpenInitialize(worldName string) error {
	saveDir := "saves"
	if err := os.MkdirAll(saveDir, 0755); err != nil {
		return err
	}

	dbPath := filepath.Join(saveDir, fmt.Sprintf("%s.fv", worldName))

	// Função interna para conectar com parâmetros otimizados
	connect := func(path string) (*gorm.DB, error) {
		// Parâmetros de conexão:
		// _journal_mode=WAL: Permite leituras concorrentes durante escritas
		// _busy_timeout=10000: Espera até 10s em vez de retornar "database is locked"
		// _synchronous=NORMAL: Equilíbrio entre performance e segurança
		dsn := fmt.Sprintf("%s?_journal_mode=WAL&_busy_timeout=10000&_synchronous=NORMAL", path)
		return gorm.Open(sqlite.Open(dsn), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Silent),
		})
	}

	db, err := connect(dbPath)
	if err != nil {
		log.Printf("[Persistence] ERRO ao conectar no SQLite: %v. Tentando reset do banco...", err)
		return s.resetCorruptDatabase(dbPath, worldName)
	}

	// Verificação de Integridade Rápida
	var integrity string
	if err := db.Raw("PRAGMA integrity_check").Scan(&integrity).Error; err != nil || integrity != "ok" {
		log.Printf("[Persistence] Banco CORROMPIDO Detectado: %s (%v). Iniciando auto-reset...", integrity, err)
		sqlDB, _ := db.DB()
		if sqlDB != nil {
			sqlDB.Close()
		}
		return s.resetCorruptDatabase(dbPath, worldName)
	}

	// Migração automática das tabelas
	if err := db.AutoMigrate(&ChunkModel{}, &WorldMetadata{}, &MaterialModel{}); err != nil {
		return fmt.Errorf("falha na migração do banco: %w", err)
	}

	s.Mu.Lock()
	s.DB = db
	s.Mu.Unlock()

	// Salva metadados iniciais
	db.Save(&WorldMetadata{Key: "FormatVersion", Value: fmt.Sprint(CurrentFormatVersion)})
	db.Save(&WorldMetadata{Key: "WorldName", Value: worldName})

	log.Printf("[Persistence] Banco de dados SQLite aberto e íntegro: %s", dbPath)
	return nil
}

// resetCorruptDatabase renomeia o arquivo corrompido e tenta criar um novo
func (s *MapDataStore) resetCorruptDatabase(dbPath, worldName string) error {
	backupPath := dbPath + ".corrupt_" + time.Now().Format("20060102_150405")
	log.Printf("[Persistence] Renomeando banco corrompido para: %s", backupPath)

	// Fecha conexões ativas se existirem
	if s.DB != nil {
		sqlDB, _ := s.DB.DB()
		if sqlDB != nil {
			sqlDB.Close()
		}
	}

	// Tenta renomear. Se falhar porque o arquivo está em uso, retornamos erro fatal.
	if err := os.Rename(dbPath, backupPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("não foi possível mover banco corrompido (arquivo em uso?): %w", err)
	}

	// Tenta reconectar em modo limpo (isso criará um novo arquivo)
	dsn := fmt.Sprintf("%s?_journal_mode=WAL&_busy_timeout=10000&_synchronous=NORMAL", dbPath)
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return fmt.Errorf("falha crítica: não foi possível criar novo banco após reset: %w", err)
	}

	if err := db.AutoMigrate(&ChunkModel{}, &WorldMetadata{}, &MaterialModel{}); err != nil {
		return fmt.Errorf("falha na migração do banco novo: %w", err)
	}

	s.Mu.Lock()
	s.DB = db
	s.Mu.Unlock()

	log.Printf("[Persistence] Novo banco de dados criado com sucesso: %s", dbPath)
	return nil
}

// SaveChunk salva um único chunk no banco de dados SQLite.
func (s *MapDataStore) SaveChunk(chunk *Chunk) error {
	if s.DB == nil {
		return fmt.Errorf("banco de dados não inicializado")
	}

	// Chunks vazios (Ar/Céu) não possuem tiles — pular serialização GOB
	// pois o GOB do Go não aceita nil dentro de arrays fixos ([16][16]*Tile).
	var data []byte
	if !chunk.IsEmpty {
		var buf bytes.Buffer
		enc := gob.NewEncoder(&buf)

		// Agrupa todos os dados para serialização única
		cData := chunkData{
			Tiles:             chunk.Tiles,
			Plants:            chunk.Plants,
			Buildings:         chunk.Buildings,
			Items:             chunk.Items,
			ConstructionItems: chunk.ConstructionItems,
			SpatterPile:       chunk.SpatterPile,
			Engravings:        chunk.Engravings,
		}

		if err := enc.Encode(cData); err != nil {
			log.Printf("[Persistence] ERRO Crítico GOB: %v", err)
			return err
		}
		data = buf.Bytes()
	}

	id := fmt.Sprintf("%d_%d_%d", chunk.Origin.X, chunk.Origin.Y, chunk.Origin.Z)
	model := ChunkModel{
		ID:      id,
		X:       chunk.Origin.X,
		Y:       chunk.Origin.Y,
		Z:       chunk.Origin.Z,
		Data:    data,
		MTime:   chunk.MTime,
		IsEmpty: chunk.IsEmpty,
	}

	// Upsert (Cria ou Atualiza)
	err := s.DB.Save(&model).Error
	if err != nil {
		log.Printf("[Persistence] ERRO ao salvar chunk %s: %v", id, err)
	} else {
		// log.Printf("[Persistence] Chunk %s salvo com sucesso", id)
		chunk.IsDirty = false
	}
	return err
}

// LoadChunk tenta carregar um chunk específico do banco de dados.
func (s *MapDataStore) LoadChunk(origin util.DFCoord) (*Chunk, error) {
	if s.DB == nil {
		return nil, fmt.Errorf("banco de dados não inicializado")
	}

	id := fmt.Sprintf("%d_%d_%d", origin.X, origin.Y, origin.Z)
	var model ChunkModel
	if err := s.DB.First(&model, "id = ?", id).Error; err != nil {
		return nil, err // Retorna error se não encontrar
	}

	var tiles [16][16]*Tile
	var plants []dfproto.PlantDetail
	var buildings []dfproto.BuildingInstance
	var items []dfproto.Item
	var constrItems []dfproto.MatPair
	var spatter []dfproto.SpatterPile
	var engravings []dfproto.Engraving

	if !model.IsEmpty && len(model.Data) > 0 {
		dec := gob.NewDecoder(bytes.NewReader(model.Data))
		// Tenta novo formato (chunkData)
		var cData chunkData
		if err := dec.Decode(&cData); err == nil {
			tiles = cData.Tiles
			plants = cData.Plants
			buildings = cData.Buildings
			items = cData.Items
			constrItems = cData.ConstructionItems
			spatter = cData.SpatterPile
			engravings = cData.Engravings
		} else {
			// Fallback: Tenta formato antigo (apenas tiles)
			decOld := gob.NewDecoder(bytes.NewReader(model.Data))
			if err := decOld.Decode(&tiles); err != nil {
				return nil, fmt.Errorf("falha ao decodificar dados do chunk %s: %v", id, err)
			}
		}
	}

	chunk := &Chunk{
		Origin:            origin,
		Tiles:             tiles,
		Plants:            plants,
		Buildings:         buildings,
		Items:             items,
		ConstructionItems: constrItems,
		SpatterPile:       spatter,
		Engravings:        engravings,
		MTime:             model.MTime,
		IsEmpty:           model.IsEmpty,
	}

	// Se for um bloco conhecido por estar vazio, abortamos links nos tiles, ele não deve possuir nenhum.
	if !model.IsEmpty {
		// Re-conecta os tiles ao container
		for x := 0; x < 16; x++ {
			for y := 0; y < 16; y++ {
				if tile := chunk.Tiles[x][y]; tile != nil {
					tile.container = s
				}
			}
		}
	}

	return chunk, nil
}

// GetChunkCount retorna a quantidade total de chunks gravados no disco SQLite.
// Usado na inicialização para decidir se compensa ligar um full-scan em background.
func (s *MapDataStore) GetChunkCount() (int64, error) {
	if s.DB == nil {
		return 0, fmt.Errorf("banco de dados não inicializado")
	}

	var count int64
	err := s.DB.Model(&ChunkModel{}).Count(&count).Error
	return count, err
}

// Save (Legacy Override) agora é apenas um wrapper que salva todos os chunks em memória.
func (s *MapDataStore) Save(worldName string) (int, error) {
	s.Mu.Lock()
	if s.DB == nil {
		if err := s.OpenInitialize(worldName); err != nil {
			s.Mu.Unlock()
			return 0, err
		}
	}

	// Coleta uma lista dos chunks sujos para salvar fora do lock
	var dirtyChunks []*Chunk
	for _, chunk := range s.Chunks {
		if chunk.IsDirty {
			dirtyChunks = append(dirtyChunks, chunk)
		}
	}
	s.Mu.Unlock() // Libera o lock para não travar o jogo durante o IO

	if len(dirtyChunks) == 0 {
		return 0, nil
	}

	// Serializa o acesso ao banco — impede "database is locked"
	s.dbMu.Lock()
	defer s.dbMu.Unlock()

	count := 0
	// Usa uma transaction para agrupar todas as escritas em uma operação atômica.
	// Isso é MUITO mais rápido e elimina "database is locked" entre goroutines.
	err := s.DB.Transaction(func(tx *gorm.DB) error {
		for _, chunk := range dirtyChunks {
			// Serialização GOB inline (igual ao SaveChunk, mas usando tx)
			var data []byte
			if !chunk.IsEmpty {
				var buf bytes.Buffer
				enc := gob.NewEncoder(&buf)
				cData := chunkData{
					Tiles:             chunk.Tiles,
					Plants:            chunk.Plants,
					Buildings:         chunk.Buildings,
					Items:             chunk.Items,
					ConstructionItems: chunk.ConstructionItems,
				}
				if err := enc.Encode(cData); err != nil {
					log.Printf("[Persistence] ERRO Crítico GOB: %v", err)
					continue // Pula este chunk, não aborta a transaction
				}
				data = buf.Bytes()
			}

			id := fmt.Sprintf("%d_%d_%d", chunk.Origin.X, chunk.Origin.Y, chunk.Origin.Z)
			model := ChunkModel{
				ID:      id,
				X:       chunk.Origin.X,
				Y:       chunk.Origin.Y,
				Z:       chunk.Origin.Z,
				Data:    data,
				MTime:   chunk.MTime,
				IsEmpty: chunk.IsEmpty,
			}

			if err := tx.Save(&model).Error; err != nil {
				log.Printf("[Persistence] ERRO ao salvar chunk %s: %v", id, err)
				continue
			}
			chunk.IsDirty = false
			count++
		}
		return nil
	})

	if err != nil {
		log.Printf("[Persistence] ERRO na transaction: %v", err)
	}

	return count, err
}

// Load (Legacy Override) inicializa o banco e pré-carrega o que for necessário.
func (s *MapDataStore) Load(worldName string) error {
	return s.OpenInitialize(worldName)
}

// GobEncode e GobDecode removidos pois agora o campo container é privado.
// O GOB lida automaticamente com a struct se não houver loops em campos exportados.
