package server

import (
	"encoding/json"
	"fmt"
	"github.com/cbartram/hearthhub-mod-api/server/service"
	amqp "github.com/rabbitmq/amqp091-go"
	log "github.com/sirupsen/logrus"
	"net/http"
	"os"
	"sync"

	"github.com/gorilla/websocket"
)

// Message represents the structure of messages being passed
type Message struct {
	Type      string      `json:"type"`
	Content   interface{} `json:"content"`
	DiscordId string      `json:"discord_id"`
}

// Client represents a WebSocket client connection
type Client struct {
	conn      *websocket.Conn
	discordId string
}

// WebSocketManager handles multiple WebSocket connections
type WebSocketManager struct {
	clients    map[*Client]bool
	broadcast  chan Message
	register   chan *Client
	unregister chan *Client
	mutex      sync.Mutex
}

// NewWebSocketManager creates a new WebSocket manager
func NewWebSocketManager() *WebSocketManager {
	return &WebSocketManager{
		clients:    make(map[*Client]bool),
		broadcast:  make(chan Message),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

// Run Listens to go routine channels for websocket events when clients
// connect, disconnect, or broadcast a message. This function keeps track
// of client state like who is connected and disconnected
func (manager *WebSocketManager) Run() {
	for {
		select {
		case client := <-manager.register:
			manager.mutex.Lock()
			manager.clients[client] = true
			manager.mutex.Unlock()
			log.Infof("client connected with discord ID: %s", client.discordId)

		case client := <-manager.unregister:
			if _, ok := manager.clients[client]; ok {
				manager.mutex.Lock()
				delete(manager.clients, client)
				client.conn.Close()
				manager.mutex.Unlock()
				log.Infof("client disconnected with discord ID: %s", client.discordId)
			}

		case message := <-manager.broadcast:
			manager.mutex.Lock()
			for client := range manager.clients {
				// Only send message if it matches the client's discord ID
				if client.discordId == message.DiscordId {
					err := client.conn.WriteJSON(message)
					if err != nil {
						log.Errorf("error broadcasting to client (%s): %v", client.discordId, err)
						client.conn.Close()
						delete(manager.clients, client)
					}
				}
			}
			manager.mutex.Unlock()
		}
	}
}

func (manager *WebSocketManager) HandleWebSocket(user *service.CognitoUser, w http.ResponseWriter, r *http.Request) {
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true // Allow all origins in development
		},
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Error upgrading connection: %v", err)
		return
	}

	client := &Client{
		conn:      conn,
		discordId: user.DiscordID,
	}

	manager.register <- client

	defer func() {
		manager.unregister <- client
	}()

	// Read messages
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			break
		}
	}
}

func (manager *WebSocketManager) ConsumeRabbitMQ() {
	// Connect to RabbitMQ
	credentials := fmt.Sprintf("%s:%s", os.Getenv("RABBITMQ_DEFAULT_USER"), os.Getenv("RABBITMQ_DEFAULT_PASS"))
	conn, err := amqp.Dial(fmt.Sprintf("amqp://%s@%s/", credentials, os.Getenv("RABBITMQ_BASE_URL")))
	if err != nil {
		log.Errorf("failed to connect to RabbitMQ: %v", err)
		return
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		log.Errorf("failed to open channel: %v", err)
	}
	defer ch.Close()

	// Declare exchange
	err = ch.ExchangeDeclare(
		"valheim-server-status", // exchange name
		"topic",                 // exchange type
		true,                    // durable
		false,                   // auto-deleted
		false,                   // internal
		false,                   // no-wait
		nil,                     // arguments
	)
	if err != nil {
		log.Fatalf("failed to declare exchange: %v", err)
	}

	// Declare queue
	q, err := ch.QueueDeclare(
		"valheim-server", // queue name
		false,            // durable
		true,             // delete when unused
		false,            // exclusive
		false,            // no-wait
		nil,              // arguments
	)
	if err != nil {
		log.Errorf("failed to declare queue: %v", err)
	}

	// Bind queue to exchange
	err = ch.QueueBind(
		q.Name,                  // queue name
		"#",                     // routing key
		"valheim-server-status", // exchange
		false,
		nil,
	)
	if err != nil {
		log.Errorf("failed to bind queue: %v", err)
	}

	msgs, err := ch.Consume(
		q.Name, // queue
		"",     // consumer
		true,   // auto-ack
		false,  // exclusive
		false,  // no-local
		false,  // no-wait
		nil,    // args
	)
	if err != nil {
		log.Errorf("failed to register consumer: %v", err)
	}

	log.Infof("successfully connected to RabbitMQ and waiting for messages...")

	// Handle incoming messages
	for msg := range msgs {
		var message Message
		if err := json.Unmarshal(msg.Body, &message); err != nil {
			log.Printf("Error unmarshaling message: %v", err)
			continue
		}
		manager.broadcast <- message
	}
}
