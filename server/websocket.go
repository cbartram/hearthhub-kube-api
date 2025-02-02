package server

import (
	"encoding/json"
	"fmt"
	amqp "github.com/rabbitmq/amqp091-go"
	log "github.com/sirupsen/logrus"
	"net/http"
	"os"
	"sync"

	"github.com/gorilla/websocket"
)

// Message represents the structure of messages being passed
type Message struct {
	Type    string      `json:"type"`
	Content interface{} `json:"content"`
}

// WebSocketManager handles multiple WebSocket connections
type WebSocketManager struct {
	clients    map[*websocket.Conn]bool
	broadcast  chan Message
	register   chan *websocket.Conn
	unregister chan *websocket.Conn
	mutex      sync.Mutex
}

// NewWebSocketManager creates a new WebSocket manager
func NewWebSocketManager() *WebSocketManager {
	return &WebSocketManager{
		clients:    make(map[*websocket.Conn]bool),
		broadcast:  make(chan Message),
		register:   make(chan *websocket.Conn),
		unregister: make(chan *websocket.Conn),
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

		case client := <-manager.unregister:
			if _, ok := manager.clients[client]; ok {
				manager.mutex.Lock()
				delete(manager.clients, client)
				client.Close()
				manager.mutex.Unlock()
			}

		case message := <-manager.broadcast:
			manager.mutex.Lock()
			for client := range manager.clients {
				err := client.WriteJSON(message)
				if err != nil {
					log.Errorf("error broadcasting to client: %s  err: %v", client.RemoteAddr(), err)
					client.Close()
					delete(manager.clients, client)
				}
			}
			manager.mutex.Unlock()
		}
	}
}

func (manager *WebSocketManager) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
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

	manager.register <- conn

	defer func() {
		manager.unregister <- conn
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
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		log.Errorf("failed to open channel: %v", err)
	}
	defer ch.Close()

	// Declare exchange
	err = ch.ExchangeDeclare(
		"messages", // exchange name
		"fanout",   // exchange type
		true,       // durable
		false,      // auto-deleted
		false,      // internal
		false,      // no-wait
		nil,        // arguments
	)
	if err != nil {
		log.Fatalf("Failed to declare exchange: %v", err)
	}

	// Declare queue
	q, err := ch.QueueDeclare(
		"valheim-server", // queue name
		false,            // durable
		true,             // delete when unused
		true,             // exclusive
		false,            // no-wait
		nil,              // arguments
	)
	if err != nil {
		log.Errorf("failed to declare queue: %v", err)
	}

	// Bind queue to exchange
	err = ch.QueueBind(
		q.Name,     // queue name
		"",         // routing key
		"messages", // exchange
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
