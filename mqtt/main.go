package main

import (
	"context"
	"log"
	"time"

	"github.com/goichi-dev/goichi"
	"github.com/goichi-dev/goichi/middleware"
	"github.com/goichi-dev/goichi/protocol"
	"github.com/goichi-dev/goichi/protocol/mqtt"
)

func main() {
	// No port multiplexing here: the broker gets its own TCP port (1883) so any
	// standard MQTT client can connect without a cmux-aware handshake.
	app := goichi.New(goichi.Config{
		Server: goichi.ServerConfig{AppName: "MQTTExample"},
	})
	app.Use(middleware.Logger())
	app.Use(middleware.Recover())

	broker := mqtt.NewMQTTBroker(mqtt.MQTTConfig{
		ProtocolConfig: protocol.ProtocolConfig{Enabled: true, Port: 1883},
		AllowAnonymous: true,
		KeepAlive:      60,
		MaxConnections: 5000,
	})
	app.RegisterProtocol(broker)

	// MQTTBroker.Subscribe is currently a no-op in the framework — the broker
	// cannot observe its own traffic yet. Watch topics with a real client:
	//
	//	mosquitto_sub -h 127.0.0.1 -t 'sensors/#' -t 'status/#' -v

	// Publish a heartbeat so a subscribed client sees traffic immediately.
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			_ = broker.Publish(context.Background(), "status/heartbeat", []byte(time.Now().Format(time.RFC3339)), 0, true)
		}
	}()

	// Publish from REST: POST /publish?topic=sensors/temp with the payload as body.
	app.POST("/publish", func(c *goichi.Context) error {
		topic := c.QueryDefault("topic", "sensors/demo")
		body := c.RequestCtx.PostBody()
		if len(body) == 0 {
			return c.BadRequest("empty body")
		}
		if err := broker.Publish(c.Context(), topic, body, 0, false); err != nil {
			return c.InternalError(err.Error())
		}
		return c.Ok(map[string]any{"topic": topic, "clients": broker.GetClientCount()})
	}).Summary("Publish an MQTT message").Tags("MQTT")

	app.GET("/mqtt/stats", func(c *goichi.Context) error {
		return c.Ok(map[string]any{
			"clients": broker.GetClientCount(),
			"running": broker.IsRunning(),
			"info":    broker.GetInfo(),
		})
	}).Summary("Broker stats").Tags("MQTT")

	log.Println("MQTT example -> mqtt://127.0.0.1:1883  (REST on http://127.0.0.1:8080)")
	if err := app.ListenGraceful("127.0.0.1:8080"); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
