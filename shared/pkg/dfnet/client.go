package dfnet

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"FortressVision/shared/pkg/dfproto"
)

const (
	// Magic strings do handshake do DFHack
	ClientMagic = "DFHack?\n\x01\x00\x00\x00"
	ServerMagic = "DFHack!\n\x01\x00\x00\x00"

	// IDs de RPC fixos do protocolo core
	RPC_REPLY_RESULT = -1
	RPC_REPLY_FAIL   = -2
	RPC_REPLY_TEXT   = -3
	RPC_REQUEST_QUIT = -4
)

// RawClient gerencia a conexão de baixo nível e o transporte RPC.
// Equivalente ao RemoteClientDF-Net.
type RawClient struct {
	conn      net.Conn
	reader    *bufio.Reader
	methodIDs map[string]int16
	mutex     sync.Mutex
}

// NewRawClient conecta ao DFHack e realiza o handshake inicial.
func NewRawClient(address string) (*RawClient, error) {
	conn, err := net.DialTimeout("tcp", address, 15*time.Second)
	if err != nil {
		return nil, fmt.Errorf("falha na conexão TCP: %v", err)
	}

	c := &RawClient{
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

func (c *RawClient) Close() {
	if c.conn != nil {
		c.conn.Close()
	}
}

func (c *RawClient) handshake() error {
	if _, err := c.conn.Write([]byte(ClientMagic)); err != nil {
		return err
	}

	buf := make([]byte, len(ServerMagic))
	if _, err := io.ReadFull(c.reader, buf); err != nil {
		return err
	}

	if string(buf) != ServerMagic {
		return fmt.Errorf("handshake falhou: magic string inválida")
	}

	return nil
}

// BindMethod registra um método de um plugin e retorna seu ID numérico.
func (c *RawClient) BindMethod(method, inputMsg, outputMsg, plugin string) (int16, error) {
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

	// ID 0 é fixo para o CoreBindRequest no protocolo do DFHack
	replyData, err := c.CallRaw(0, reqData)
	if err != nil {
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

// SuspendGame pausa o Dwarf Fortress. Útil para leituras consistentes de mapa.
func (c *RawClient) SuspendGame() error {
	_, err := c.CallRaw(2, []byte{}) // ID 2 é fixo para CoreSuspend
	return err
}

// ResumeGame retoma a execução do Dwarf Fortress.
func (c *RawClient) ResumeGame() error {
	_, err := c.CallRaw(3, []byte{}) // ID 3 é fixo para CoreResume
	return err
}

// RunCommand executa um comando de console no DFHack.
func (c *RawClient) RunCommand(command string, args []string) error {
	req := dfproto.CoreRunCommandRequest{
		Command:   command,
		Arguments: args,
	}
	data, _ := req.Marshal()
	_, err := c.CallRaw(1, data) // ID 1 é fixo para CoreRunCommand
	return err
}

// CallRaw executa uma chamada RPC bruta enviando o ID e o payload binário.
func (c *RawClient) CallRaw(id int16, data []byte) ([]byte, error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// Header: ID(2) + Padding(2) + Size(4)
	header := make([]byte, 8)
	binary.LittleEndian.PutUint16(header[0:], uint16(id))
	binary.LittleEndian.PutUint32(header[4:], uint32(len(data)))

	if _, err := c.conn.Write(header); err != nil {
		return nil, err
	}
	if _, err := c.conn.Write(data); err != nil {
		return nil, err
	}

	c.conn.SetReadDeadline(time.Now().Add(20 * time.Second))

	for {
		if _, err := io.ReadFull(c.reader, header); err != nil {
			return nil, err
		}

		replyID := int16(binary.LittleEndian.Uint16(header[0:]))
		size := int32(binary.LittleEndian.Uint32(header[4:]))

		body := make([]byte, size)
		if _, err := io.ReadFull(c.reader, body); err != nil {
			return nil, err
		}

		switch replyID {
		case RPC_REPLY_RESULT:
			return body, nil
		case RPC_REPLY_FAIL:
			return nil, fmt.Errorf("RPC erro: código %d", size)
		case RPC_REPLY_TEXT:
			// Notificações de log do DFHack (opcional: rotear para um logger)
			continue
		default:
			return nil, fmt.Errorf("ID de resposta inesperado: %d", replyID)
		}
	}
}
