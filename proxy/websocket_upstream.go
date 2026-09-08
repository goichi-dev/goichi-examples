package main

import (
	"log"
	"net/http"

	"github.com/coder/websocket"
)

// websocketBackend is the upstream behind the proxy's /ws route: it echoes each
// message back with a prefix, which is enough to show that the upgrade and the
// frames afterwards both survive the hop through the proxy.
func websocketBackend(addr string) {
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
			// The handshake arrives from the proxy, so the browser Origin check
			// would compare against the wrong host. A real upstream behind a
			// trusted proxy checks the forwarded origin instead.
			InsecureSkipVerify: true,
		})
		if err != nil {
			log.Printf("ws upstream: accept: %v", err)
			return
		}
		defer conn.CloseNow()

		ctx := r.Context()
		for {
			typ, data, err := conn.Read(ctx)
			if err != nil {
				return
			}
			if err := conn.Write(ctx, typ, append([]byte("echo: "), data...)); err != nil {
				return
			}
		}
	})
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Printf("ws upstream stopped: %v", err)
	}
}
