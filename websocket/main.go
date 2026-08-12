package main

import (
	"log"

	"github.com/goichi-dev/goichi"
	"github.com/goichi-dev/goichi/middleware"
	"github.com/goichi-dev/goichi/protocol"
	"github.com/goichi-dev/goichi/protocol/websocket"
)

func main() {
	app := goichi.New(goichi.Config{
		Server: goichi.ServerConfig{
			AppName:                "WebSocketExample",
			EnablePortMultiplexing: true,
		},
	})
	app.Use(middleware.Logger())
	app.Use(middleware.Recover())

	ws := websocket.NewWebSocketServer(websocket.WebSocketConfig{
		ProtocolConfig: protocol.ProtocolConfig{Enabled: true, Port: 8080},
		Path:           "/ws",
		// Same-origin only when empty. Use []string{"*"} to allow any origin.
		AllowedOrigins: []string{"http://127.0.0.1:8080"},
	})

	ws.OnConnect = func(conn *websocket.WebSocketConn) {
		log.Printf("[WS] connected: %s", conn.ID)
	}
	ws.OnMessage = func(conn *websocket.WebSocketConn, messageType int, data []byte) {
		log.Printf("[WS] %s -> %s", conn.ID, data)
		_ = ws.SendToClient(conn.ID, messageType, append([]byte("echo: "), data...))
	}
	ws.OnDisconnect = func(conn *websocket.WebSocketConn, err error) {
		log.Printf("[WS] disconnected: %s (%v)", conn.ID, err)
	}

	app.RegisterProtocol(ws)

	// Push a message to every connected client.
	app.POST("/broadcast", func(c *goichi.Context) error {
		body := c.RequestCtx.PostBody()
		if len(body) == 0 {
			return c.BadRequest("empty body")
		}
		if err := ws.Broadcast(body); err != nil {
			return c.InternalError(err.Error())
		}
		return c.Ok(map[string]int{"clients": len(ws.GetConnections())})
	}).Summary("Broadcast to all WebSocket clients").Tags("WebSocket")

	app.GET("/clients", func(c *goichi.Context) error {
		ids := make([]string, 0, len(ws.GetConnections()))
		for id := range ws.GetConnections() {
			ids = append(ids, id)
		}
		return c.Ok(ids)
	}).Summary("List connected clients").Tags("WebSocket")

	log.Println("WebSocket example -> ws://127.0.0.1:8080/ws")
	if err := app.ListenGraceful("127.0.0.1:8080"); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
