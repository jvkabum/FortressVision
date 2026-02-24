package mapdata

import (
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

const CurrentFormatVersion = 2

// OpenInitialize abre (ou cria) o banco de dados SQLite para o mundo e roda migrações.
func (s *MapDataStore) OpenInitialize(worldName string) error {
	if err := os.MkdirAll("saves", 0755); err != nil {
		return err
	}

	dbPath := filepath.Join("saves", fmt.Sprintf("%s.fv", worldName))

	// Configuramos o logger para ser silencioso em produção
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return fmt.Errorf("falha ao conectar no SQLite: %w", err)
	}

	// Migração automática das tabelas
	if err := db.AutoMigrate(&ChunkModel{}, &WorldMetadata{}, &MaterialModel{}); err != nil {
		return fmt.Errorf("falha na migração do banco: %w", err)
	}

	s.DB = db

	// Salva metadados iniciais
	db.Save(&WorldMetadata{Key: "FormatVersion", Value: fmt.Sprint(CurrentFormatVersion)})
	db.Save(&WorldMetadata{Key: "WorldName", Value: worldName})

	log.Printf("[Persistence] Banco de dados SQLite aberto: %s", dbPath)
	return nil
}

// SaveChunk salva um único chunk no banco de dados SQLite.
func (s *MapDataStore) SaveChunk(chunk *Chunk) error {
	if s.DB == nil {
		return fmt.Errorf("banco de dados não inicializado")
	}

	// Serializa os tiles do chunk em bytes (GOB)
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(chunk.Tiles); err != nil {
		log.Printf("[Persistence] ERRO Crítico GOB: %v", err)
		return err
	}

	id := fmt.Sprintf("%d_%d_%d", chunk.Origin.X, chunk.Origin.Y, chunk.Origin.Z)
	model := ChunkModel{
		ID:    id,
		X:     chunk.Origin.X,
		Y:     chunk.Origin.Y,
		Z:     chunk.Origin.Z,
		Data:  buf.Bytes(),
		MTime: chunk.MTime,
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

	// Deserializa os tiles
	var tiles [16][16]*Tile
	dec := gob.NewDecoder(bytes.NewReader(model.Data))
	if err := dec.Decode(&tiles); err != nil {
		return nil, err
	}

	chunk := &Chunk{
		Origin: origin,
		Tiles:  tiles,
		MTime:  model.MTime,
	}

	// Re-conecta os tiles ao container
	for x := 0; x < 16; x++ {
		for y := 0; y < 16; y++ {
			if tile := chunk.Tiles[x][y]; tile != nil {
				tile.container = s
			}
		}
	}

	return chunk, nil
}

// Save (Legacy Override) agora é apenas um wrapper que salva todos os chunks em memória.
func (s *MapDataStore) Save(worldName string) error {
	s.Mu.Lock()
	if s.DB == nil {
		if err := s.OpenInitialize(worldName); err != nil {
			s.Mu.Unlock()
			return err
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
		return nil
	}

	log.Printf("[Persistence] Iniciando salvamento assíncrono em SQLite... (Chunks sujos: %d)", len(dirtyChunks))
	count := 0
	for _, chunk := range dirtyChunks {
		if err := s.SaveChunk(chunk); err == nil {
			count++
		}
	}
	log.Printf("[Persistence] Salvamento concluído: %d chunks persistidos.", count)

	return nil
}

// Load (Legacy Override) inicializa o banco e pré-carrega o que for necessário.
func (s *MapDataStore) Load(worldName string) error {
	return s.OpenInitialize(worldName)
}

// GobEncode e GobDecode removidos pois agora o campo container é privado.
// O GOB lida automaticamente com a struct se não houver loops em campos exportados.
