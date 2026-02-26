package dfclient

import (
	"FortressVision/shared/pkg/dfnet"
	"FortressVision/shared/pkg/dfproto"
	"fmt"
)

// RemoteFortressService abstrai as chamadas do plugin RemoteFortressReader.
// Equivalente ao RemoteClientLocal.
type RemoteFortressService struct {
	net *dfnet.RawClient
}

func NewRemoteFortressService(net *dfnet.RawClient) *RemoteFortressService {
	return &RemoteFortressService{net: net}
}

// Suspend pausa o jogo via rede.
func (s *RemoteFortressService) Suspend() error {
	return s.net.SuspendGame()
}

// Resume retoma o jogo via rede.
func (s *RemoteFortressService) Resume() error {
	return s.net.ResumeGame()
}

// RunCommand executa um comando de console.
func (s *RemoteFortressService) RunCommand(cmd string, args []string) error {
	return s.net.RunCommand(cmd, args)
}

const pluginName = "RemoteFortressReader"

// Assinaturas de métodos para Bind automático (Contracts)
var signatures = map[string][2]string{
	"GetTiletypeList":    {"dfproto.EmptyMessage", "RemoteFortressReader.TiletypeList"},
	"GetViewInfo":        {"dfproto.EmptyMessage", "RemoteFortressReader.ViewInfo"},
	"GetBlockList":       {"RemoteFortressReader.BlockRequest", "RemoteFortressReader.BlockList"},
	"GetMaterialList":    {"dfproto.EmptyMessage", "RemoteFortressReader.MaterialList"},
	"GetPlantList":       {"RemoteFortressReader.BlockRequest", "RemoteFortressReader.PlantList"},
	"GetUnitList":        {"dfproto.EmptyMessage", "RemoteFortressReader.UnitList"},
	"GetMapInfo":         {"dfproto.EmptyMessage", "RemoteFortressReader.MapInfo"},
	"GetWorldMapCenter":  {"dfproto.EmptyMessage", "RemoteFortressReader.WorldMap"},
	"GetBuildingDefList": {"dfproto.EmptyMessage", "RemoteFortressReader.BuildingList"},
	"GetBuildingList":    {"dfproto.EmptyMessage", "RemoteFortressReader.BuildingInstanceList"},
	"GetLanguage":        {"dfproto.EmptyMessage", "RemoteFortressReader.Language"},
}

func (s *RemoteFortressService) call(method string, reqMarshaler interface{ Marshal() ([]byte, error) }, respUnmarshaler interface{ Unmarshal([]byte) error }) error {
	sig, ok := signatures[method]
	if !ok {
		return fmt.Errorf("método desconhecido: %s", method)
	}

	id, err := s.net.BindMethod(method, sig[0], sig[1], pluginName)
	if err != nil {
		return err
	}

	reqData, err := reqMarshaler.Marshal()
	if err != nil {
		return err
	}

	respData, err := s.net.CallRaw(id, reqData)
	if err != nil {
		return err
	}

	return respUnmarshaler.Unmarshal(respData)
}

// --- Wrappers de Serviço ---

func (s *RemoteFortressService) GetTiletypeList() (*dfproto.TiletypeList, error) {
	resp := &dfproto.TiletypeList{}
	err := s.call("GetTiletypeList", &dfproto.EmptyMessage{}, resp)
	return resp, err
}

func (s *RemoteFortressService) GetMaterialList() (*dfproto.MaterialList, error) {
	resp := &dfproto.MaterialList{}
	err := s.call("GetMaterialList", &dfproto.EmptyMessage{}, resp)
	return resp, err
}

// GetMapInfo retorna informações básicas sobre o tamanho e posição do mapa atual.
func (s *RemoteFortressService) GetMapInfo() (*dfproto.MapInfo, error) {
	var resp dfproto.MapInfo
	err := s.call("GetMapInfo", &dfproto.EmptyMessage{}, &resp)
	return &resp, err
}

func (s *RemoteFortressService) GetUnitList() (*dfproto.UnitList, error) {
	resp := &dfproto.UnitList{}
	err := s.call("GetUnitList", &dfproto.EmptyMessage{}, resp)
	return resp, err
}

func (s *RemoteFortressService) GetViewInfo() (*dfproto.ViewInfo, error) {
	resp := &dfproto.ViewInfo{}
	err := s.call("GetViewInfo", &dfproto.EmptyMessage{}, resp)
	return resp, err
}

func (s *RemoteFortressService) GetWorldMapCenter() (*dfproto.WorldMap, error) {
	resp := &dfproto.WorldMap{}
	err := s.call("GetWorldMapCenter", &dfproto.EmptyMessage{}, resp)
	return resp, err
}

func (s *RemoteFortressService) GetBlockList(req *dfproto.BlockRequest) (*dfproto.BlockList, error) {
	resp := &dfproto.BlockList{}
	err := s.call("GetBlockList", req, resp)
	return resp, err
}

func (s *RemoteFortressService) GetPlantList() (*dfproto.PlantRawList, error) {
	resp := &dfproto.PlantRawList{}
	err := s.call("GetPlantList", &dfproto.EmptyMessage{}, resp)
	return resp, err
}

func (s *RemoteFortressService) GetBuildingDefList() (*dfproto.BuildingList, error) {
	resp := &dfproto.BuildingList{}
	err := s.call("GetBuildingDefList", &dfproto.EmptyMessage{}, resp)
	return resp, err
}

func (s *RemoteFortressService) GetBuildingList() (*dfproto.BuildingInstanceList, error) {
	resp := &dfproto.BuildingInstanceList{}
	err := s.call("GetBuildingList", &dfproto.EmptyMessage{}, resp)
	return resp, err
}

func (s *RemoteFortressService) GetLanguage() (*dfproto.Language, error) {
	resp := &dfproto.Language{}
	err := s.call("GetLanguage", &dfproto.EmptyMessage{}, resp)
	return resp, err
}
