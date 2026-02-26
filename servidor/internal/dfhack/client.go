// Package dfhack fornece um cliente de alto nível para comunicação com DFHack.
// Abstrai a complexidade do protocolo TCP/protobuf em métodos simples.
package dfhack

import (
	"fmt"
	"sync"
	"time"

	"FortressVision/shared/pkg/dfclient"
	"FortressVision/shared/pkg/dfnet"
	"FortressVision/shared/pkg/dfproto"
)

// Client é uma fachada fina que gerencia a vida útil da conexão.
type Client struct {
	Service   *dfclient.RemoteFortressService
	raw       *dfnet.RawClient
	connected bool
	mu        sync.RWMutex

	lastReconnect time.Time
	reconnectMu   sync.Mutex

	// Cache de dados estáticos
	TiletypeList *dfproto.TiletypeList
	MaterialList *dfproto.MaterialList
	PlantRawList *dfproto.PlantRawList
	MapInfo      *dfproto.MapInfo

	address string

	// Instant Z-Sync: Priorização de nível por demanda do cliente
	OverrideInterestZ int32
	LastOverrideTime  time.Time
}

// NewClient cria e conecta um novo cliente usando a arquitetura dfnet/dfclient.
func NewClient(address string) (*Client, error) {
	c := &Client{
		address: address,
	}
	if err := c.connect(); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Client) connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.raw != nil {
		c.raw.Close()
	}

	raw, err := dfnet.NewRawClient(c.address)
	if err != nil {
		c.connected = false
		return fmt.Errorf("dfnet: %w", err)
	}

	c.raw = raw
	c.Service = dfclient.NewRemoteFortressService(raw)
	c.connected = true
	return nil
}

// Reconnect limpa a conexão atual e tenta estabelecer uma nova com limite de frequência.
// Útil após timeouts ou erros de protocolo que deixam o socket dessincronizado.
func (c *Client) Reconnect(reason error) error {
	c.reconnectMu.Lock()
	defer c.reconnectMu.Unlock()

	// Throttle: não reconecta mais de uma vez a cada 2 segundos para evitar "storms"
	if time.Since(c.lastReconnect) < 2*time.Second {
		return nil
	}

	fmt.Printf("[dfhack] Resetando conexão (Motivo: %v). Reiniciando socket com %s...\n", reason, c.address)
	err := c.connect()
	if err == nil {
		c.lastReconnect = time.Now()
	}
	return err
}

func (c *Client) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.raw != nil {
		c.raw.Close()
		c.connected = false
	}
}

func (c *Client) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}

func (c *Client) FetchStaticData() error {
	fmt.Println("[dfhack] Carregando dados estáticos do mundo...")

	// Removida Suspensão Global (Fase 12): Causava deadlocks em mundos grandes
	// se o tempo de transferência protobuf excedesse o timeout ou segurasse o loop do jogo.

	var err error

	c.TiletypeList, err = c.Service.GetTiletypeList()
	if err != nil {
		return err
	}
	fmt.Printf("  → %d tiletypes carregados\n", len(c.TiletypeList.TiletypeList))

	c.MaterialList, err = c.Service.GetMaterialList()
	if err != nil {
		return err
	}
	fmt.Printf("  → %d materiais carregados\n", len(c.MaterialList.MaterialList))

	// Novos dados baseados no Armok Vision
	buildings, err := c.Service.GetBuildingDefList()
	if err == nil {
		fmt.Printf("  → %d definições de prédios carregadas\n", len(buildings.BuildingList))
	}

	lang, err := c.Service.GetLanguage()
	if err == nil {
		fmt.Printf("  → Suporte a traduções OK\n")
		_ = lang // Por enquanto apenas validando a conexão
	}

	c.MapInfo, err = c.Service.GetMapInfo()
	if err != nil {
		return err
	}
	fmt.Printf("  → Mundo: %s\n", c.MapInfo.WorldNameEn)

	c.PlantRawList, err = c.Service.GetPlantList()
	if err != nil {
		fmt.Printf(" [!] Erro ao carregar PlantRaws: %v\n", err)
	}
	return nil
}

// --- Wrappers delegados para o dfclient ---

func (c *Client) GetViewInfo() (*dfproto.ViewInfo, error) {
	res, err := c.Service.GetViewInfo()
	if err != nil {
		c.Reconnect(err)
	}
	return res, err
}

func (c *Client) GetWorldMapCenter() (*dfproto.WorldMap, error) {
	res, err := c.Service.GetWorldMapCenter()
	if err != nil {
		c.Reconnect(err)
	}
	return res, err
}

func (c *Client) GetUnitList() (*dfproto.UnitList, error) {
	res, err := c.Service.GetUnitList()
	if err != nil {
		c.Reconnect(err)
	}
	return res, err
}

func (c *Client) GetBuildingList() (*dfproto.BuildingInstanceList, error) {
	res, err := c.Service.GetBuildingList()
	if err != nil {
		c.Reconnect(err)
	}
	return res, err
}

func (c *Client) GetBlockList(minX, minY, minZ, maxX, maxY, maxZ, blocksNeeded int32) (*dfproto.BlockList, error) {
	c.mu.RLock()
	info := c.MapInfo
	c.mu.RUnlock()

	req := &dfproto.BlockRequest{
		BlocksNeeded: blocksNeeded,
		MinX:         minX, MaxX: maxX,
		MinY: minY, MaxY: maxY,
		MinZ: minZ, MaxZ: maxZ,
		ForceReload: true,
	}

	// Tradução Global -> Local para o DFHack 53.10
	if info != nil {
		req.MinX -= info.BlockPosX
		req.MaxX -= info.BlockPosX
		req.MinY -= info.BlockPosY
		req.MaxY -= info.BlockPosY
		req.MinZ -= info.BlockPosZ
		req.MaxZ -= info.BlockPosZ
	}

	res, err := c.Service.GetBlockList(req)
	if err != nil {
		c.Reconnect(err)
		return nil, err
	}

	// Tradução Local -> Global para o FortressVision
	if res != nil && info != nil {
		for i := range res.MapBlocks {
			// Nota: No DFHack, MapX e MapY já são reportados como coordenadas TILE globais (region_x + local_tile_x).
			// Somar BlockPosX/Y aqui causaria desalinhamento (dobraria o offset ou somaria blocos a tiles).
			// Apenas o MapZ precisa ser ajustado se estivermos usando o sistema de índices locais (0-N).
			res.MapBlocks[i].MapZ += info.BlockPosZ
		}
	}

	return res, err
}

func (c *Client) GetInterestZ() int32 {
	c.mu.RLock()
	// Z-Sync de Alta Prioridade: Se o cliente requisitou um nível específico nos últimos 10 segundos,
	// o scanner deve focar as varreduras exclusivamente nesse nível para carregamento instantâneo.
	if time.Since(c.LastOverrideTime) < 10*time.Second {
		z := c.OverrideInterestZ
		c.mu.RUnlock()
		return z
	}
	c.mu.RUnlock()

	view, err := c.GetViewInfo()
	if err != nil {
		return 0
	}
	// Armok Vision utiliza Z + 1 para alinhar o nível visual com o DF
	return view.ViewPosZ + 1
}

func (c *Client) SetInterestZ(z int32) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.OverrideInterestZ = z
	c.LastOverrideTime = time.Now()
}
