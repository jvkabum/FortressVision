// Package dfhack fornece um cliente de alto nível para comunicação com DFHack.
// Abstrai a complexidade do protocolo TCP/protobuf em métodos simples.
package dfhack

import (
	"fmt"
	"sync"

	"FortressVision/shared/pkg/dfproto"
	"FortressVision/shared/pkg/protocol"
)

// Client é o cliente de alto nível para o RemoteFortressReader.
type Client struct {
	rpc       *protocol.Client
	connected bool
	mu        sync.RWMutex

	// Cache de dados estáticos
	TiletypeList *dfproto.TiletypeList
	MaterialList *dfproto.MaterialList
	PlantRawList *dfproto.PlantRawList
	MapInfo      *dfproto.MapInfo
}

// NewClient cria e conecta um novo cliente DFHack.
func NewClient(address string) (*Client, error) {
	rpc, err := protocol.NewClient(address)
	if err != nil {
		return nil, fmt.Errorf("falha ao conectar ao DFHack em %s: %w", address, err)
	}

	c := &Client{
		rpc:       rpc,
		connected: true,
	}

	return c, nil
}

// Close fecha a conexão com o DFHack.
func (c *Client) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.rpc != nil {
		c.rpc.Close()
		c.connected = false
	}
}

// IsConnected retorna se o cliente está conectado.
func (c *Client) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}

// IsConnectedString retorna uma string representativa do status de conexão.
func (c *Client) IsConnectedString() string {
	if c.IsConnected() {
		return "ON"
	}
	return "OFF"
}

// --- Métodos de inicialização (dados estáticos) ---

// FetchStaticData carrega tiletypes, materiais e info do mapa.
func (c *Client) FetchStaticData() error {
	var err error

	// TiletypeList (necessário para interpretar os IDs dos tiles)
	c.TiletypeList, err = c.GetTiletypeList()
	if err != nil {
		return fmt.Errorf("GetTiletypeList: %w", err)
	}
	fmt.Printf("  → %d tiletypes carregados\n", len(c.TiletypeList.TiletypeList))

	// MaterialList (necessário para cores)
	c.MaterialList, err = c.GetMaterialList()
	if err != nil {
		return fmt.Errorf("GetMaterialList: %w", err)
	}
	fmt.Printf("  → %d materiais carregados\n", len(c.MaterialList.MaterialList))

	// MapInfo (tamanho do mapa, nome do mundo)
	c.MapInfo, err = c.GetMapInfo()
	if err != nil {
		return fmt.Errorf("GetMapInfo: %w", err)
	}
	fmt.Printf("  → Mundo: %s (%s)\n", c.MapInfo.WorldNameEn, c.MapInfo.WorldName)
	fmt.Printf("  → Tamanho: %dx%dx%d blocos\n",
		c.MapInfo.BlockSizeX, c.MapInfo.BlockSizeY, c.MapInfo.BlockSizeZ)

	// PlantRawList (definições de árvores e plantas)
	c.PlantRawList, err = c.GetPlantList()
	if err != nil {
		// Log mas não falha o boot se der erro nas plantas
		fmt.Printf(" [!] Erro ao carregar PlantRaws: %v\n", err)
	} else {
		fmt.Printf("  → %d espécies de plantas carregadas\n", len(c.PlantRawList.PlantRaws))
	}

	return nil
}

// --- Métodos RPC do RemoteFortressReader ---

const pluginName = "RemoteFortressReader"

// GetTiletypeList obtém a lista de definições de tiletypes.
func (c *Client) GetTiletypeList() (*dfproto.TiletypeList, error) {
	req := &dfproto.EmptyMessage{}
	resp := &dfproto.TiletypeList{}
	if err := c.rpc.Call("GetTiletypeList", pluginName, req, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// GetMaterialList obtém a lista de materiais do jogo.
func (c *Client) GetMaterialList() (*dfproto.MaterialList, error) {
	req := &dfproto.EmptyMessage{}
	resp := &dfproto.MaterialList{}
	if err := c.rpc.Call("GetMaterialList", pluginName, req, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// GetMapInfo obtém informações sobre o mapa (tamanho, nome do mundo).
func (c *Client) GetMapInfo() (*dfproto.MapInfo, error) {
	req := &dfproto.EmptyMessage{}
	resp := &dfproto.MapInfo{}
	if err := c.rpc.Call("GetMapInfo", pluginName, req, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// GetPlantList obtém a lista de definições de plantas do jogo.
func (c *Client) GetPlantList() (*dfproto.PlantRawList, error) {
	req := &dfproto.BlockRequest{}
	resp := &dfproto.PlantRawList{}
	if err := c.rpc.Call("GetPlantList", pluginName, req, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// GetViewInfo obtém a posição atual da câmera no DF.
func (c *Client) GetViewInfo() (*dfproto.ViewInfo, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	req := &dfproto.EmptyMessage{}
	resp := &dfproto.ViewInfo{}
	if err := c.rpc.Call("GetViewInfo", pluginName, req, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// GetWorldMapCenter obtém informações sobre o centro do mundo e tempo.
func (c *Client) GetWorldMapCenter() (*dfproto.WorldMap, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	req := &dfproto.EmptyMessage{}
	resp := &dfproto.WorldMap{}
	if err := c.rpc.Call("GetWorldMapCenter", pluginName, req, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// GetUnitList obtém a lista de criaturas/unidades.
func (c *Client) GetUnitList() (*dfproto.UnitList, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	req := &dfproto.EmptyMessage{}
	resp := &dfproto.UnitList{}
	if err := c.rpc.Call("GetUnitList", pluginName, req, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// GetBlockList obtém blocos do mapa na região especificada.
func (c *Client) GetBlockList(minX, minY, minZ, maxX, maxY, maxZ, blocksNeeded int32) (*dfproto.BlockList, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	req := &dfproto.BlockRequest{
		BlocksNeeded:      blocksNeeded,
		MinX:              minX,
		MaxX:              maxX,
		MinY:              minY,
		MaxY:              maxY,
		MinZ:              minZ,
		MaxZ:              maxZ,
		RequestTiles:      true,
		RequestMaterials:  true,
		RequestLiquid:     true,
		RequestVegetation: true,
	}
	resp := &dfproto.BlockList{}
	if err := c.rpc.Call("GetBlockList", pluginName, req, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// SetPauseState pausa ou despausa o jogo.
func (c *Client) SetPauseState(paused bool) error {
	req := &dfproto.SingleBool{Value: paused}
	resp := &dfproto.EmptyMessage{}
	return c.rpc.Call("SetPauseState", pluginName, req, resp)
}

// SendDigCommand envia um comando de escavação.
func (c *Client) SendDigCommand(x, y, z int32, designation dfproto.TileDigDesignation) error {
	req := &dfproto.DigCommand{
		PosX:        x,
		PosY:        y,
		PosZ:        z,
		Designation: designation,
	}
	resp := &dfproto.EmptyMessage{}
	return c.rpc.Call("SendDigCommand", pluginName, req, resp)
}

// CheckHashes verifica e reseta hashes para forçar atualização de blocos.
func (c *Client) CheckHashes() error {
	req := &dfproto.EmptyMessage{}
	resp := &dfproto.EmptyMessage{}
	return c.rpc.Call("CheckHashes", pluginName, req, resp)
}
