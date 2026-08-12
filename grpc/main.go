package main

import (
	"log"
	"net/http"

	"github.com/goichi-dev/goichi"
	"github.com/goichi-dev/goichi/middleware"
	"github.com/goichi-dev/goichi/protocol"
	"github.com/goichi-dev/goichi/protocol/grpc"

	rawgrpc "google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
)

func main() {
	app := goichi.New(goichi.Config{
		Server: goichi.ServerConfig{
			AppName:                "GRPCExample",
			EnablePortMultiplexing: true,
		},
	})
	app.Use(middleware.Logger())
	app.Use(middleware.Recover())

	// Interceptors registered here apply to the gRPC server below.
	app.UseGRPCUnary(middleware.GRPCRecover(), middleware.GRPCLogger())
	app.UseGRPCStream(middleware.GRPCStreamRecover(), middleware.GRPCStreamLogger())

	// 1. Classic gRPC. Register your generated services inside RegisterFunc.
	grpcSrv := grpc.NewGRPCServer(grpc.GRPCConfig{
		ProtocolConfig: protocol.ProtocolConfig{Enabled: true, Port: 8080},
	})
	grpcSrv.RegisterFunc = func(s *rawgrpc.Server) {
		// pb.RegisterYourServiceServer(s, &yourServer{})
		healthpb.RegisterHealthServer(s, health.NewServer())
		reflection.Register(s)
		log.Println("[gRPC] health + reflection registered")
	}
	app.RegisterProtocol(grpcSrv)

	// 2. ConnectRPC over plain HTTP — mount generated handlers by path.
	connectSrv := grpc.NewConnectServer(grpc.GRPCConfig{
		ProtocolConfig: protocol.ProtocolConfig{Enabled: true, Port: 8080},
	})
	connectSrv.Register("/greet.v1.GreetService/", http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"greeting":"hello from ConnectRPC"}`))
		},
	))
	app.RegisterProtocol(connectSrv)

	// Reading JWT claims inside a gRPC handler:
	//
	//	claims := grpc.GetClaims(ctx)
	//	user, _ := claims["username"].(string)

	log.Println("gRPC example      -> grpc://127.0.0.1:8080 (grpcurl -plaintext 127.0.0.1:8080 list)")
	log.Println("ConnectRPC example-> http://127.0.0.1:8080/greet.v1.GreetService/Greet")
	if err := app.ListenGraceful("127.0.0.1:8080"); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
