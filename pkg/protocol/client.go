package protocol

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"FortressVision/pkg/dfproto"
)

const (
	// Magic strings do handshake
	ClientMagic = "DFHack?\n\x01\x00\x00\x00"
	ServerMagic = "DFHack!\n\x01\x00\x00\x00"

	// IDs de RPC fixos
	RPC_REPLY_RESULT = -1
	RPC_REPLY_FAIL   = -2
	RPC_REPLY_TEXT   = -3
	RPC_REQUEST_QUIT = -4
)

// Assinaturas de métodos conhecidos (para Bind correto)
var MethodSignatures = map[string][2]string{
	"GetTiletypeList":   {"dfproto.EmptyMessage", "RemoteFortressReader.TiletypeList"},
	"GetViewInfo":       {"dfproto.EmptyMessage", "RemoteFortressReader.ViewInfo"},
	"GetBlockList":      {"RemoteFortressReader.BlockRequest", "RemoteFortressReader.BlockList"},
	"GetMaterialList":   {"dfproto.EmptyMessage", "RemoteFortressReader.MaterialList"},
	"GetPlantList":      {"RemoteFortressReader.BlockRequest", "RemoteFortressReader.PlantList"},
	"GetUnitList":       {"dfproto.EmptyMessage", "RemoteFortressReader.UnitList"},
	"CheckHashes":       {"dfproto.EmptyMessage", "dfproto.EmptyMessage"},
	"GetMapInfo":        {"dfproto.EmptyMessage", "RemoteFortressReader.MapInfo"},
	"GetWorldMapCenter": {"dfproto.EmptyMessage", "RemoteFortressReader.WorldMap"},
}

// Client gerencia a conexão com o DFHack.
type Client struct {
	conn      net.Conn
	reader    *bufio.Reader
	methodIDs map[string]int16
	mutex     sync.Mutex
}

// NewClient conecta ao DFHack e realiza o handshake.
func NewClient(address string) (*Client, error) {
	conn, err := net.DialTimeout("tcp", address, 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("falha ao conectar: %v", err)
	}

	c := &Client{
		conn:      conn,
		reader:    bufio.NewReader(conn),
		methodIDs: make(map[string]int16),
	}

	if err := c.handshake(); err != nil {
		conn.Close()
		return nil, err
	}

	return c, nil
}

func (c *Client) Close() {
	if c.conn != nil {
		c.conn.Close()
	}
}

// handshake realiza a troca de magic strings.
func (c *Client) handshake() error {
	// Envia ClientMagic
	if _, err := c.conn.Write([]byte(ClientMagic)); err != nil {
		return err
	}

	// Lê e valida ServerMagic (12 bytes)
	buf := make([]byte, len(ServerMagic))
	if _, err := io.ReadFull(c.reader, buf); err != nil {
		return err
	}

	if string(buf) != ServerMagic {
		return fmt.Errorf("protocolo inválido: esperado %q, recebeu %q", ServerMagic, buf)
	}

	return nil
}

// BindMethod obtém o ID de um método RPC.
func (c *Client) BindMethod(method, inputMsg, outputMsg, plugin string) (int16, error) {
	key := method + ":" + plugin
	if id, ok := c.methodIDs[key]; ok {
		return id, nil
	}

	req := dfproto.CoreBindRequest{
		Method:    method,
		InputMsg:  inputMsg,
		OutputMsg: outputMsg,
		Plugin:    plugin,
	}

	reqData, _ := req.Marshal()

	// O ID para BindMethod é 0 (Core Protocol)
	fmt.Printf("Tentando BindMethod: %s (Plugin: %s) [In: %s, Out: %s]...\n", method, plugin, inputMsg, outputMsg) // Debug log
	replyData, err := c.CallRaw(0, reqData)
	if err != nil {
		fmt.Printf("Erro no BindMethod %s: %v\n", method, err) // Debug log
		return 0, err
	}

	var reply dfproto.CoreBindReply
	if err := reply.Unmarshal(replyData); err != nil {
		return 0, err
	}

	id := int16(reply.AssignedID)
	c.methodIDs[key] = id
	return id, nil
}

// Call invoca um método RPC. Automaticamente faz bind se necessário.
func (c *Client) Call(method, plugin string, reqMarshaler interface{ Marshal() ([]byte, error) }, respUnmarshaler interface{ Unmarshal([]byte) error }) error {
	var id int16
	var err error

	// Verifica se já temos o ID
	key := method + ":" + plugin
	if existingID, ok := c.methodIDs[key]; ok {
		id = existingID
	} else {
		// Se não, faz bind.
		// Tentar descobrir tipos de mensagem
		inType := ""
		outType := ""
		if sig, known := MethodSignatures[method]; known {
			inType = sig[0]
			outType = sig[1]
		}

		id, err = c.BindMethod(method, inType, outType, plugin)
		if err != nil {
			return err
		}
	}

	reqData, err := reqMarshaler.Marshal()
	if err != nil {
		return err
	}

	replyData, err := c.CallRaw(id, reqData)
	if err != nil {
		return err
	}

	return respUnmarshaler.Unmarshal(replyData)
}

func (c *Client) CallRaw(id int16, data []byte) ([]byte, error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// Header: ID (2 bytes) + Padding (2 bytes) + Size (4 bytes)
	header := make([]byte, 8)
	binary.LittleEndian.PutUint16(header[0:], uint16(id))
	// header[2:4] padding = 0
	binary.LittleEndian.PutUint32(header[4:], uint32(len(data)))

	// Escreve header + data
	if _, err := c.conn.Write(header); err != nil {
		return nil, err
	}
	if _, err := c.conn.Write(data); err != nil {
		return nil, err
	}

	// Lê resposta
	// Timeout de 30 segundos para leitura
	c.conn.SetReadDeadline(time.Now().Add(30 * time.Second))

	for {
		// Lê header da resposta
		if _, err := io.ReadFull(c.reader, header); err != nil {
			return nil, fmt.Errorf("timeout/erro lendo header RPC: %v", err)
		}

		replyID := int16(binary.LittleEndian.Uint16(header[0:]))
		size := int32(binary.LittleEndian.Uint32(header[4:]))

		// Lê corpo da mensagem
		body := make([]byte, size)
		if _, err := io.ReadFull(c.reader, body); err != nil {
			return nil, err
		}

		switch replyID {
		case RPC_REPLY_RESULT:
			return body, nil
		case RPC_REPLY_FAIL:
			return nil, fmt.Errorf("RPC Falhou com código: %d", size)
		case RPC_REPLY_TEXT:
			// Consome notificações de texto e continua esperando o resultado
			fmt.Printf("DFHack Log: %q\n", body)
			continue
		default:
			return nil, fmt.Errorf("RPC ID inesperado: %d", replyID)
		}
	}
}
