package main

import (
	"log"

	"github.com/goichi-dev/goichi"
	"github.com/goichi-dev/goichi/middleware"
	"github.com/goichi-dev/goichi/protocol"
	"github.com/goichi-dev/goichi/protocol/graphql"
)

func main() {
	app := goichi.New(goichi.Config{
		Server: goichi.ServerConfig{
			AppName:                "GraphQLExample",
			EnablePortMultiplexing: true,
		},
	})
	app.Use(middleware.Logger())
	app.Use(middleware.Recover())

	gql := graphql.NewGraphQLServer(graphql.GraphQLConfig{
		ProtocolConfig: protocol.ProtocolConfig{Enabled: true, Port: 8080},
		QueryPath:      "/graphql",
		PlaygroundPath: "/graphql/playground",
		MaxQueryDepth:  10,
		MaxComplexity:  500,
		// The playground implies schema introspection — keep both off in production.
		EnablePlayground: true,
	})

	// Schema is a gqlgen ExecutableSchema. Run `go run github.com/99designs/gqlgen
	// generate` in your own project and assign the result here:
	//
	//	gql.Schema = generated.NewExecutableSchema(generated.Config{Resolvers: &Resolver{}})
	//
	// Left nil on purpose so this example runs with no codegen step — the server
	// then answers every query with a fixed {"data":{"ping":"pong"}}.
	gql.Schema = nil

	app.RegisterProtocol(gql)

	log.Println("GraphQL example -> http://127.0.0.1:8080/graphql")
	log.Println("Playground      -> http://127.0.0.1:8080/graphql/playground")
	if err := app.ListenGraceful("127.0.0.1:8080"); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
