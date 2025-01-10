package main

import (
	"encoding/json"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"log"
	"net/http"
	"sync"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type Sessions struct {
	sync.Mutex
	m map[string]*websocket.Conn
}

func (s *Sessions) Add(c *websocket.Conn) {
	s.Lock()
	defer s.Unlock()
	s.m[c.RemoteAddr().String()] = c
}

func (s *Sessions) Remove(c *websocket.Conn) {
	s.Lock()
	defer s.Unlock()
	delete(s.m, c.RemoteAddr().String())
}

func (s *Sessions) Notify(message []byte) {
	s.Lock()
	defer s.Unlock()
	for _, conn := range s.m {
		if err := conn.WriteMessage(websocket.TextMessage, message); err != nil {
			log.Println("ws write error:", err)
			return
		}
	}
}

var sessions = Sessions{m: make(map[string]*websocket.Conn)}

func WebSocketHandler(c *gin.Context) {
	ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Println(err)
		return
	}

	sessions.Add(ws)
	defer sessions.Remove(ws)

	for {
		messageType, _, err := ws.ReadMessage()
		if err != nil {
			break
		}
		if messageType == websocket.CloseMessage {
			break
		}
	}

	ws.Close()
}

func initWs(group *gin.RouterGroup) gin.IRoutes {
	return group.GET("/ws", WebSocketHandler)
}

func GenerateMessage(messageType string, data gin.H) []byte {
	body := gin.H{
		"message_type": messageType,
		"data":         data,
	}
	bodyJson, _ := json.Marshal(body)
	return bodyJson
}
