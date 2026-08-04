package liquid

import (
	"fmt"
	"log"
	"os"
	"sync"
	"time"
)

// ========================================================================
// TRACER — Sistema de Rastreamento do Pipeline de Líquidos
//
// Este módulo registra CADA PASSO da vida de um bloco de líquido:
//   ETAPA 1: Rede     → Chunk recebido do servidor (WaterLevel no GOB)
//   ETAPA 2: Store    → Tile inserido no MapDataStore em memória
//   ETAPA 3: Enqueue  → Chunk enfileirado para meshing
//   ETAPA 4: Mesher   → Tile processado pelo BlockMesher (WaterLevel lido)
//   ETAPA 5: Geometry → Geometria de líquido gerada (vértices)
//   ETAPA 6: Upload   → Modelo de líquido enviado à GPU (LiquidModel)
//   ETAPA 7: Draw     → Modelo desenhado na tela (PASS 2)
//
// Todas as mensagens vão para o arquivo LIQUID_TRACE.log na pasta do cliente.
// ========================================================================

var (
	traceLogger  *log.Logger
	traceFile    *os.File
	traceOnce    sync.Once
	traceEnabled bool = true // Setar false para desativar sem remover código
)

// InitTracer inicializa o arquivo de log do tracer. Deve ser chamado uma vez no boot.
func InitTracer() {
	traceOnce.Do(func() {
		if !traceEnabled {
			return
		}
		var err error
		traceFile, err = os.OpenFile("LIQUID_TRACE.log", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0666)
		if err != nil {
			log.Printf("[Tracer] ERRO: Não foi possível criar LIQUID_TRACE.log: %v", err)
			traceEnabled = false
			return
		}
		traceLogger = log.New(traceFile, "", 0 /* sem prefixo — controlamos o formato */)
		traceLogger.Println("=== LIQUID PIPELINE TRACER INICIADO ===")
		traceLogger.Printf("=== Horário: %s ===", time.Now().Format("2006-01-02 15:04:05"))
		traceLogger.Println("=== Acompanhe cada etapa do pipeline de líquidos ===")
		traceLogger.Println("")
		log.Println("[Tracer] LIQUID_TRACE.log criado com sucesso!")
	})
}

// CloseTracer fecha o arquivo de trace.
func CloseTracer() {
	if traceFile != nil {
		traceFile.Close()
	}
}

// Trace registra uma mensagem no pipeline tracer.
func Trace(etapa int, tag string, format string, args ...interface{}) {
	if !traceEnabled || traceLogger == nil {
		return
	}
	msg := fmt.Sprintf(format, args...)
	ts := time.Now().Format("15:04:05.000")
	traceLogger.Printf("[%s] ETAPA %d (%s): %s", ts, etapa, tag, msg)
}

// TraceNetwork registra quando um chunk com água chega pela rede.
// Chamar em: network.go → processChunk()
func TraceNetwork(chunkX, chunkY, chunkZ int32, totalTilesComAgua int, totalTilesComMagma int) {
	Trace(1, "REDE", "Chunk (%d,%d,%d) recebido. Tiles com água: %d, com magma: %d",
		chunkX, chunkY, chunkZ, totalTilesComAgua, totalTilesComMagma)
}

// TraceStore registra quando um tile com líquido é inserido no MapDataStore.
// Chamar em: network.go → processChunk() após inserir no store
func TraceStore(x, y, z, waterLevel, magmaLevel int32, hidden bool) {
	Trace(2, "STORE", "Tile (%d,%d,%d) no MapDataStore. WaterLevel=%d, MagmaLevel=%d, Hidden=%v",
		x, y, z, waterLevel, magmaLevel, hidden)
}

// TraceEnqueue registra quando um chunk é enfileirado para meshing.
// Chamar em: app_network.go → OnMapChunk callback
func TraceEnqueue(originX, originY, originZ int32, mtime int64) {
	Trace(3, "ENQUEUE", "Chunk (%d,%d,%d) enfileirado para meshing. MTime=%d",
		originX, originY, originZ, mtime)
}

// TraceMesher registra quando o mesher processa um tile e encontra líquido.
// Chamar em: block_mesher.go → runGreedyMesher2D() no if tile.WaterLevel > 0
func TraceMesher(x, y, z, waterLevel, magmaLevel int32, hidden bool) {
	Trace(4, "MESHER", "Tile (%d,%d,%d) processado. Water=%d, Magma=%d, Hidden=%v",
		x, y, z, waterLevel, magmaLevel, hidden)
}

// TraceHeartbeat registra que o mesher está vivo e processando um chunk.
func TraceHeartbeat(x, y, z int32, face int32) {
	// Log reduzido para não inundar o arquivo
	if x%32 == 0 && y%32 == 0 {
		Trace(0, "DEBUG", "MESHER VIVO em Chunk (%d,%d,%d) para face ID %d", x, y, z, face)
	}
}

// TraceGeometry registra quando a geometria de líquido é gerada.
// Chamar em: liquid/water.go → GenerateWaterGeometry()
func TraceGeometry(x, y, z, level int32, vertexCount int) {
	Trace(5, "GEOMETRY", "Geometria gerada para (%d,%d,%d). Level=%d, Vértices=%d",
		x, y, z, level, vertexCount)
}

// TraceUpload registra quando o modelo de líquido é carregado na GPU.
// Chamar em: renderer.go → UploadResult() quando HasLiquid = true
func TraceUpload(originX, originY, originZ int32, vertexCount int) {
	Trace(6, "GPU", "LiquidModel carregado na GPU para chunk (%d,%d,%d). Vértices=%d",
		originX, originY, originZ, vertexCount)
}

// TraceDraw registra quando o modelo de líquido é desenhado na tela.
// Chamar em: renderer.go → Draw() PASS 2
func TraceDraw(originX, originY, originZ int32) {
	Trace(7, "DRAW", "LiquidModel renderizado na tela: chunk (%d,%d,%d)",
		originX, originY, originZ)
}

// TraceDebug permite registrar mensagens genéricas de depuração no log de líquidos.
func TraceDebug(msg string) {
	Trace(0, "DEBUG", msg)
}
