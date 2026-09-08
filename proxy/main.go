// Command proxy demonstrates Goichi as a reverse proxy: forwarding routes to
// HTTP upstreams with load balancing and health checks, passing WebSocket
// connections through, and proxying a raw TCP service at layer 4.
//
// So the example runs on its own, it starts the upstreams it proxies to: two
// HTTP backends, a WebSocket echo server, and a line-based TCP service standing
// in for a database. In a real deployment those live on other hosts and only the
// proxy configuration below is yours.
package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/goichi-dev/goichi"
	"github.com/goichi-dev/goichi/middleware"
	"github.com/goichi-dev/goichi/protocol"
	"github.com/goichi-dev/goichi/proxy"
	"github.com/valyala/fasthttp"
)

const (
	backendA   = "127.0.0.1:9201"
	backendB   = "127.0.0.1:9202"
	wsBackend  = "127.0.0.1:9203"
	tcpBackend = "127.0.0.1:9301"
)

func main() {
	startUpstreams()

	app := goichi.New(goichi.Config{
		Server: goichi.ServerConfig{AppName: "ProxyExample"},
	})

	app.Use(middleware.Logger())
	app.Use(middleware.RequestID())
	app.EnableHealthCheck("/health")

	// --- 1. Single upstream -------------------------------------------------
	//
	// Everything under /single is forwarded to one backend with the prefix
	// removed, so /single/info arrives upstream as /info. The handler is nil:
	// the proxy middleware answers the request and never calls next.
	app.ANY("/single/*", nil, proxy.New(proxy.Config{
		Target:      "http://" + backendA,
		StripPrefix: "/single",
	}))

	// --- 2. Load balancing across a pool ------------------------------------
	//
	// Requests alternate between the two backends. Health checks probe /health
	// every few seconds and drop a failing backend out of rotation until it
	// recovers, and MaxRetries sends a request that hit a dead target to
	// another one instead of failing.
	app.ANY("/balanced/*", nil, proxy.Balance(
		[]string{backendA, backendB},
		proxy.Config{
			StripPrefix: "/balanced",
			Balancer:    proxy.RoundRobin,
			MaxRetries:  2,
			HealthCheck: proxy.HealthCheck{
				Enable:   true,
				Path:     "/health",
				Interval: 5 * time.Second,
			},
		},
	))

	// --- 3. Weighted routing with an unreachable target ---------------------
	//
	// The first target is not listening, which is what health checking is for:
	// after FailThreshold probes it leaves the pool and every request lands on
	// the backend that is up. Weight makes a target chosen proportionally more
	// often than its peers.
	app.ANY("/failover/*", nil, proxy.New(proxy.Config{
		Targets: []*proxy.Target{
			{URL: "127.0.0.1:9299"},    // nothing listening here
			{URL: backendB, Weight: 2}, // healthy, picked twice as often
		},
		StripPrefix: "/failover",
		Balancer:    proxy.LeastConn,
		MaxRetries:  2,
		HealthCheck: proxy.HealthCheck{Enable: true, Path: "/health", Interval: 3 * time.Second},
	}))

	// --- 4. Rewriting a proxied request and response -------------------------
	//
	// ModifyRequest runs before the request goes upstream and ModifyResponse
	// before the response goes back, which is where header injection, tenant
	// routing and response tagging belong.
	app.ANY("/tenant/*", nil, proxy.New(proxy.Config{
		Target:      "http://" + backendA,
		StripPrefix: "/tenant",
		ModifyRequest: func(c *goichi.Context, req *fasthttp.Request) {
			req.Header.Set("X-Tenant-ID", c.QueryDefault("tenant", "public"))
		},
		ModifyResponse: func(c *goichi.Context, res *fasthttp.Response) {
			res.Header.Set("X-Served-Via", "goichi-proxy")
		},
	}))

	// --- 5. WebSocket passthrough -------------------------------------------
	//
	// With WebSocket enabled the proxy replays the upgrade handshake upstream
	// and then pipes raw frames both ways for the life of the connection.
	app.ANY("/ws", nil, proxy.New(proxy.Config{
		Target:    "http://" + wsBackend,
		WebSocket: true,
	}))

	// --- 6. TCP proxy at layer 4 --------------------------------------------
	//
	// This fronts a byte-stream service the way you would front Postgres, MySQL
	// or Redis. It is a protocol server, so registering it makes it start and
	// stop with the app. The hooks see raw bytes: the framework never parses
	// the wire protocol, so the same proxy works for any of them.
	db := proxy.NewTCPProxy(proxy.TCPConfig{
		ProtocolConfig: protocol.ProtocolConfig{
			Enabled: true,
			Address: "127.0.0.1",
			Port:    9300,
		},
		Name:           "db-proxy",
		Target:         tcpBackend,
		Balancer:       proxy.LeastConn,
		MaxConnections: 200,
		IdleTimeout:    5 * time.Minute,
		HealthCheck:    proxy.HealthCheck{Enable: true, Interval: 5 * time.Second},

		// Connection-level policy, before an upstream is dialled. A rejected
		// connection never reaches the database.
		OnConnect: func(c *proxy.TCPConn) error {
			host, _, err := net.SplitHostPort(c.Client.RemoteAddr().String())
			if err == nil && host != "127.0.0.1" && host != "::1" {
				return fmt.Errorf("connection from %s is not allowed", host)
			}
			c.Set("opened", time.Now())
			return nil
		},

		// Each chunk read from the client, before it is forwarded. Returning an
		// error closes the connection, so nothing reaches the upstream.
		//
		// Chunks are byte-level, not message-level: a chunk boundary is not a
		// protocol boundary. A hook that needs whole packets must buffer them
		// itself, typically in c.Locals.
		OnClientData: func(c *proxy.TCPConn, b []byte) ([]byte, error) {
			if strings.Contains(strings.ToUpper(string(b)), "DROP TABLE") {
				return nil, fmt.Errorf("blocked a DROP TABLE statement")
			}
			return b, nil
		},

		// Accounting for the connection once it closes.
		OnClose: func(c *proxy.TCPConn, sent, received int64) {
			var since time.Duration
			if t, ok := c.Get("opened").(time.Time); ok {
				since = time.Since(t).Round(time.Millisecond)
			}
			log.Printf("[db-proxy] closed after %v: sent=%dB received=%dB", since, sent, received)
		},
	})
	app.RegisterProtocol(db)

	// Report what the proxy pools look like right now.
	app.GET("/proxy/stats", func(c *goichi.Context) error {
		active, total := db.Stats()
		targets := make([]map[string]any, 0)
		for _, t := range db.Targets() {
			targets = append(targets, map[string]any{
				"url":     t.URL,
				"healthy": t.Healthy(),
				"active":  t.Active(),
			})
		}
		return c.JSON(map[string]any{
			"tcp": map[string]any{
				"active_connections": active,
				"total_connections":  total,
				"targets":            targets,
			},
		})
	})

	log.Println("proxy example on http://127.0.0.1:8080  (db proxy on tcp://127.0.0.1:9300)")
	log.Fatal(app.Listen(":8080"))
}

// startUpstreams runs the services this example proxies to. A real deployment
// would have these somewhere else entirely.
func startUpstreams() {
	go httpBackend("backend-a", backendA)
	go httpBackend("backend-b", backendB)
	go websocketBackend(wsBackend)
	go tcpService(tcpBackend)
	time.Sleep(300 * time.Millisecond)
}

// httpBackend echoes which instance answered and the forwarding headers it
// received, so you can see what the proxy actually sent.
func httpBackend(name, addr string) {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w,
			`{"backend":%q,"path":%q,"x_forwarded_for":%q,"x_forwarded_proto":%q,"x_tenant_id":%q}`,
			name, r.URL.Path,
			r.Header.Get("X-Forwarded-For"),
			r.Header.Get("X-Forwarded-Proto"),
			r.Header.Get("X-Tenant-ID"))
	})
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Printf("upstream %s stopped: %v", name, err)
	}
}

// tcpService is a line-based byte-stream service standing in for a database: it
// answers each line it receives in upper case.
func tcpService(addr string) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		log.Printf("tcp upstream stopped: %v", err)
		return
	}
	for {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		go func(conn net.Conn) {
			defer conn.Close()
			r := bufio.NewReader(conn)
			for {
				line, err := r.ReadString('\n')
				if err != nil {
					return
				}
				fmt.Fprintf(conn, "OK: %s", strings.ToUpper(line))
			}
		}(conn)
	}
}
