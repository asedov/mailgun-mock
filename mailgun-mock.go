package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

type message map[string][]string

type client struct {
	conn   *websocket.Conn // The websocket connection.
	send   chan []byte     // Buffered channel of outbound messages.
	isOpen bool
}

var config = struct {
	apiKey     string
	webhookKey string
	webhookUrl string
}{}

var upgrader = websocket.Upgrader{}

var messages = struct {
	sync.RWMutex
	items map[string]message
}{items: make(map[string]message)} // Messages queue

var clients = struct {
	sync.RWMutex
	items map[*client]bool
}{items: make(map[*client]bool)} // Registered clients.

func broadcast(action string, id *string, data interface{}) {
	jsn, _ := json.Marshal(&struct {
		Action string      `json:"action"`
		Id     *string     `json:"id"`
		Data   interface{} `json:"data"`
	}{
		Action: action,
		Id:     id,
		Data:   data,
	})

	clients.RLock()
	for c := range clients.items {
		if c.isOpen {
			c.send <- jsn
		}
	}
	clients.RUnlock()
}

func disconnect(c *client) {
	if c.isOpen {
		c.isOpen = false
		close(c.send)
		_ = c.conn.Close()
	}

	clients.Lock()
	_, ok := clients.items[c]
	if ok {
		delete(clients.items, c)
	}
	clients.Unlock()
}

func ws(w http.ResponseWriter, r *http.Request) {
	con, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	c := &client{
		conn:   con,
		send:   make(chan []byte, 32),
		isOpen: true,
	}

	clients.Lock()
	clients.items[c] = true
	clients.Unlock()

	con.SetCloseHandler(func(code int, text string) error {
		disconnect(c)
		return nil
	})

	go func() {
		ticker := time.NewTicker(time.Second * 5)

		defer func() {
			ticker.Stop()
			disconnect(c)
		}()

		for {
			select {
			case msg, ok := <-c.send:
				if !ok {
					return
				}
				err = c.conn.WriteMessage(websocket.TextMessage, msg)
				if err != nil {
					return
				}
			case <-ticker.C:
				err = c.conn.WriteMessage(websocket.PingMessage, nil)
				if err != nil {
					return
				}
			}
		}
	}()

	go func() {
		for {
			_, msg, err := c.conn.ReadMessage()
			if err != nil {
				break
			}

			data := &struct {
				Action string `json:"action"`
				Id     string `json:"id"`
			}{}
			_ = json.Unmarshal(msg, data)

			if data.Action == "remove" {
				messages.Lock()
				_, ok := messages.items[data.Id]
				if ok {
					delete(messages.items, data.Id)
					broadcast("del", &data.Id, struct{}{})
				}
				messages.Unlock()
			}
		}

		disconnect(c)
	}()

	messages.RLock()
	items := messages.items
	messages.RUnlock()

	jsn, _ := json.Marshal(&struct {
		Action string      `json:"action"`
		Data   interface{} `json:"data"`
	}{
		"sync",
		items,
	})
	c.send <- jsn
}

func postMessages(w http.ResponseWriter, r *http.Request) {
	log.Printf("%s %s", r.Method, r.URL.Path)

	usr, pas, ok := r.BasicAuth()
	if !ok || usr != "api" || pas != config.apiKey {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	err := r.ParseMultipartForm(20 * 1024 * 1024)
	if err != nil {
		err = r.ParseForm()
		if err != nil {
			http.Error(w, fmt.Sprintf("could not parse request: %s", err), http.StatusBadRequest)
			return
		}
	}

	params := mux.Vars(r)

	msgId := fmt.Sprintf("%d@%s", time.Now().UnixNano(), params["domain"])

	msg := make(message)
	for key, value := range r.Form {
		msg[key] = value
		if strings.ToLower(key) == "h:message-id" {
			msgId = value[0]
		}
	}

	messages.Lock()
	messages.items[msgId] = msg
	messages.Unlock()

	broadcast("add", &msgId, &msg)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(&struct {
		Id      string `json:"id"`
		Message string `json:"message"`
	}{
		Id:      msgId,
		Message: "Queued. Thank you.",
	})
}

func main() {
	port := flag.Int("port", 80, "specify port")
	host := flag.String("host", "0.0.0.0", "specify host")
	flag.Parse()

	val, ok := os.LookupEnv("MAILGUN_API_KEY")
	if ok {
		config.apiKey = val
	}

	val, ok = os.LookupEnv("MAILGUN_WEBHOOK_KEY")
	if ok {
		config.webhookKey = val
	}

	val, ok = os.LookupEnv("MAILGUN_WEBHOOK_URL")
	if ok {
		config.webhookUrl = val
	}

	r := mux.NewRouter()
	r.HandleFunc("/ws", ws)
	r.HandleFunc("/v3/{domain}/messages", postMessages).Methods("POST")
	r.PathPrefix("/").Handler(http.FileServer(http.Dir("./public/")))

	log.Printf(`api_key: "%s"`, config.apiKey)
	log.Printf(`webhook_key: "%s"`, config.webhookKey)
	log.Printf(`webhook_url: "%s"`, config.webhookUrl)
	log.Printf("Running on http://%s:%d/", *host, *port)

	log.Fatal(http.ListenAndServe(fmt.Sprintf("%s:%d", *host, *port), r))
}
