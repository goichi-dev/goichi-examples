package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/goichi-dev/goichi"
	"github.com/goichi-dev/goichi/middleware"
	"github.com/goichi-dev/goichi/protocol"
	"github.com/goichi-dev/goichi/protocol/mcp"
)

func main() {
	// MCP speaks newline-delimited JSON-RPC over a raw TCP socket — not HTTP —
	// and always opens its own listener, so it gets its own port even when the
	// rest of the app is multiplexed.
	app := goichi.New(goichi.Config{
		Server: goichi.ServerConfig{AppName: "MCPExample"},
	})
	app.Use(middleware.Logger())
	app.Use(middleware.Recover())

	mcpSrv := mcp.NewMCPServer(mcp.MCPConfig{
		ProtocolConfig: protocol.ProtocolConfig{Enabled: true, Port: 8085},
		Timeout:        30,
	})

	_ = mcpSrv.RegisterTool(&mcp.Tool{
		Name:        "get_status",
		Description: "Return server status and uptime",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
		Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return map[string]any{"status": "running", "time": time.Now().Format(time.RFC3339)}, nil
		},
	})

	_ = mcpSrv.RegisterTool(&mcp.Tool{
		Name:        "echo",
		Description: "Echo the message argument back to the caller",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"message": map[string]any{"type": "string", "description": "Text to echo"},
			},
			"required": []string{"message"},
		},
		Handler: func(ctx context.Context, args map[string]any) (any, error) {
			msg, ok := args["message"].(string)
			if !ok {
				return nil, fmt.Errorf("message must be a string")
			}
			return msg, nil
		},
	})

	_ = mcpSrv.RegisterResource(&mcp.Resource{
		URI:         "config://app",
		Name:        "App configuration",
		Description: "Static configuration exposed to the agent",
		MimeType:    "application/json",
		Content:     map[string]any{"appName": "MCPExample", "version": "0.1.0"},
	})

	mcpSrv.RegisterPrompt(&mcp.Prompt{
		Name:        "summarize",
		Description: "Ask the model to summarize a body of text",
		Arguments: []mcp.PromptArgument{
			{Name: "text", Description: "Text to summarize", Required: true},
		},
		Handler: func(ctx context.Context, args map[string]any) (string, error) {
			return fmt.Sprintf("Summarize the following:\n\n%v", args["text"]), nil
		},
	})

	app.RegisterProtocol(mcpSrv)

	app.GET("/mcp/tools", func(c *goichi.Context) error {
		names := make([]string, 0, len(mcpSrv.GetTools()))
		for name := range mcpSrv.GetTools() {
			names = append(names, name)
		}
		return c.Ok(names)
	}).Summary("List registered MCP tools").Tags("MCP")

	log.Println("MCP example -> tcp://127.0.0.1:8085 (JSON-RPC, one request per line)")
	log.Println("REST helper -> http://127.0.0.1:8080/mcp/tools")
	if err := app.ListenGraceful("127.0.0.1:8080"); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
