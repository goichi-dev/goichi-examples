package main

import (
	"context"
	"log"
	"time"

	"github.com/goichi-dev/goichi"
	"github.com/goichi-dev/goichi-examples/internal"
	"github.com/goichi-dev/goichi/middleware"
	"github.com/goichi-dev/goichi/protocol"
	"github.com/goichi-dev/goichi/protocol/graphql"
	"github.com/goichi-dev/goichi/protocol/grpc"
	"github.com/goichi-dev/goichi/protocol/mcp"
	"github.com/goichi-dev/goichi/protocol/mqtt"
	"github.com/goichi-dev/goichi/protocol/websocket"

	rawgrpc "google.golang.org/grpc"
)

func main() {
	app := goichi.New(goichi.Config{
		Server: goichi.ServerConfig{
			AppName:                "FullProtocolExample",
			EnablePortMultiplexing: true,
		},
		Docs: goichi.DocsConfig{
			Enable:    true,
			Path:      "/docs",
			Title:     "Full Protocol Example API",
			Desc:      "REST, gRPC, ConnectRPC, GraphQL, MQTT and WebSocket on one port.",
			PageTitle: "Goichi Example Docs",
		},
	})
	app.Use(middleware.Favicon())
	app.Use(middleware.Logger())
	app.Use(middleware.Recover())
	app.EnableHealthCheck("/health")

	// 1. REST — login issues a real HS256 token signed with JWT_SECRET.
	app.POST("/login", func(c *goichi.Context) error {
		var req internal.LoginRequest
		if err := c.Bind(&req); err != nil {
			return c.BadRequest(err.Error())
		}
		token, ok := internal.IssueToken(req.Username, req.Password)
		if !ok {
			return c.Unauthorized("invalid credentials")
		}
		return c.Ok(internal.LoginResponse{Token: token})
	}).
		Summary("User Login").
		Tags("Authentication").
		RequestModel(internal.LoginRequest{}).
		ResponseModel(internal.LoginResponse{})

	// 2. gRPC
	grpcSrv := grpc.NewGRPCServer(grpc.GRPCConfig{
		ProtocolConfig: protocol.ProtocolConfig{Enabled: true, Port: 8080},
	})
	grpcSrv.RegisterFunc = func(s *rawgrpc.Server) {
		log.Println("[Example] gRPC Service Registered")
	}
	app.RegisterProtocol(grpcSrv)

	// 3. ConnectRPC (gRPC semantics over plain HTTP)
	connectSrv := grpc.NewConnectServer(grpc.GRPCConfig{
		ProtocolConfig: protocol.ProtocolConfig{Enabled: true, Port: 8080},
	})
	app.RegisterProtocol(connectSrv)

	// 4. GraphQL — see ../graphql for how to plug in a gqlgen schema.
	gqlSrv := graphql.NewGraphQLServer(graphql.GraphQLConfig{
		ProtocolConfig: protocol.ProtocolConfig{Enabled: true, Port: 8080},
	})
	app.RegisterProtocol(gqlSrv)

	// 5. MQTT broker
	mqttBroker := mqtt.NewMQTTBroker(mqtt.MQTTConfig{
		ProtocolConfig: protocol.ProtocolConfig{Enabled: true, Port: 8080},
	})
	app.RegisterProtocol(mqttBroker)
	go func() {
		time.Sleep(5 * time.Second)
		_ = mqttBroker.Publish(context.Background(), "status", []byte("broker-started"), 0, false)
	}()

	// 6. WebSocket
	wsSrv := websocket.NewWebSocketServer(websocket.WebSocketConfig{
		ProtocolConfig: protocol.ProtocolConfig{Enabled: true, Port: 8080},
		Path:           "/ws",
	})
	wsSrv.OnConnect = func(conn *websocket.WebSocketConn) {
		log.Printf("[WS] Client connected: %s", conn.ID)
	}
	wsSrv.OnMessage = func(conn *websocket.WebSocketConn, messageType int, data []byte) {
		log.Printf("[WS] Message from %s: %s", conn.ID, string(data))
		_ = wsSrv.SendToClient(conn.ID, messageType, data)
	}
	app.RegisterProtocol(wsSrv)

	// 7. MCP for AI agents — runs its own listener, hence a separate port.
	mcpSrv := mcp.NewMCPServer(mcp.MCPConfig{
		ProtocolConfig: protocol.ProtocolConfig{Enabled: true, Port: 8085},
	})
	_ = mcpSrv.RegisterTool(&mcp.Tool{
		Name:        "get_status",
		Description: "Get server status",
		Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return map[string]string{"status": "running"}, nil
		},
	})
	_ = mcpSrv.RegisterTool(&mcp.Tool{
		Name:        "echo",
		Description: "Echoes input back",
		Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return args["message"], nil
		},
	})
	app.RegisterProtocol(mcpSrv)

	log.Println("Starting Goichi Full Protocol Example on 127.0.0.1:8080 (MCP on :8085)")
	if err := app.ListenGraceful("127.0.0.1:8080"); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
