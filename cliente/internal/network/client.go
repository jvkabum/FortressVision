package network

import (
	"log"
	"sync"
	"time"

	"FortressVision/shared/proto/fvnet"
	"github.com/gorilla/websocket"
	"google.golang.org/protobuf/proto"
)

// Client gerencia a comunicação WebSocket crua com o servidor FortressVision.
// Não contém metadados do jogo ou armazenamento de mapa, atuando puramente
// como a "Estrada" física entre o servidor e o cliente modular.
type Client struct {
	conn      *websocket.Conn
	url       string
	connected bool
	mu        sync.RWMutex
	connectMu sync.Mutex

	// Sistema de Eventos: Handlers podem ser injetados pelo "Mestre de Obras"
	// para despachar mensagens a outros módulos (como o internal/world).
	OnEnvelope   func(env *fvnet.Envelope)
	OnConnect    func()
	OnDisconnect func()
}

// NewClient cria uma nova instância da Estrada de Rede.
func NewClient(url string) *Client {
	return &Client{
		url: url,
	}
}

// Connect tenta conectar ao WebSocket do servidor usando fallback/retry.
func (c *Client) Connect() error {
	dialer := websocket.Dialer{
		HandshakeTimeout: 5 * time.Second,
	}

	c.connectMu.Lock()
	defer c.connectMu.Unlock()
	if c.IsConnected() {
		return nil
	}

	var err error
	// O servidor pode levar alguns minutos para abrir o banco/cache do mundo.
	// Mantemos a janela de retry longa para evitar um cliente vazio quando ele
	// for iniciado junto com o launcher.
	maxRetries := 300
	for i := 0; i < maxRetries; i++ {
		log.Printf("[Network] Tentativa %d/%d em %s...", i+1, maxRetries, c.url)
		c.conn, _, err = dialer.Dial(c.url, nil)
		if err == nil {
			break
		}
		log.Printf("[Network] Servidor não pronto: %v. Aguardando 2s...", err)
		time.Sleep(2 * time.Second)
	}

	if err != nil {
		log.Printf("[Network] ❌ ERRO após %d tentativas: %v", maxRetries, err)
		return err
	}

	c.mu.Lock()
	c.connected = true
	c.mu.Unlock()

	log.Println("[Network] 📶 Conectado ao servidor!")
	if c.OnConnect != nil {
		c.OnConnect()
	}

	go c.readLoop()
	return nil
}

// IsConnected retorna true se a "Estrada" estiver pavimentada e livre.
func (c *Client) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}

// Send serializa e envia uma mensagem Protobuf tipada empacotada em um Envelope.
func (c *Client) Send(msgType fvnet.Envelope_Type, msg proto.Message) {
	if !c.IsConnected() {
		return
	}

	var payload []byte
	var err error
	if msg != nil {
		payload, err = proto.Marshal(msg)
		if err != nil {
			log.Printf("[Network] ❌ Erro ao serializar payload: %v", err)
			return
		}
	}

	env := &fvnet.Envelope{
		Type:    msgType,
		Payload: payload,
	}

	data, err := proto.Marshal(env)
	if err != nil {
		log.Printf("[Network] ❌ Erro ao empacotar envelope: %v", err)
		return
	}

	c.mu.Lock()
	err = c.conn.WriteMessage(websocket.BinaryMessage, data)
	c.mu.Unlock()

	if err != nil {
		log.Printf("[Network] ❌ Erro ao enviar mensagem: %v", err)
		c.disconnectLocally()
	}
}

// readLoop processa as mensagens de entrada e despacha pelo callback OnEnvelope.
func (c *Client) readLoop() {
	defer c.disconnectLocally()

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			log.Printf("[Network] 🔌 Conexão perdida: %v", err)
			break
		}

		var env fvnet.Envelope
		if err := proto.Unmarshal(message, &env); err != nil {
			log.Printf("[Network] ❌ Erro ao desempacotar envelope ignorado: %v", err)
			continue
		}

		// Despachar a correspondência (Envelope) se houver handler listando.
		if c.OnEnvelope != nil {
			c.OnEnvelope(&env)
		} else {
			// Apenas log de debug para ter certeza que algo chegou
			log.Printf("[Network] Envelope recebido mas não consumido. Type: %v", env.Type)
		}
	}
}

// disconnectLocally limpa o estado de comunicação
func (c *Client) disconnectLocally() {
	c.mu.Lock()
	wasConnected := c.connected
	c.connected = false
	c.mu.Unlock()

	if c.conn != nil {
		c.conn.Close()
	}

	if wasConnected && c.OnDisconnect != nil {
		c.OnDisconnect()
	}
}
