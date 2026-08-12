package main

import (
	"log"
	"sync"

	"github.com/goichi-dev/goichi"
	"github.com/goichi-dev/goichi-examples/internal"
	"github.com/goichi-dev/goichi/middleware"
)

var (
	mu     sync.RWMutex
	nextID = 3
	todos  = map[int]internal.Todo{
		1: {ID: 1, Title: "Read the guide", Done: true},
		2: {ID: 2, Title: "Ship something", Done: false},
	}
)

func main() {
	app := goichi.New(goichi.Config{
		Server: goichi.ServerConfig{AppName: "RESTExample"},
		Docs: goichi.DocsConfig{
			Enable: true,
			Path:   "/docs",
			Title:  "REST Example API",
			Desc:   "Routing, binding, validation, docs and JWT-protected groups.",
		},
	})

	app.Use(middleware.Logger())
	app.Use(middleware.Recover())
	app.Use(middleware.RequestID())
	app.Use(middleware.CORS())
	app.EnableHealthCheck("/health")

	// Public: exchange credentials for a token (admin / password).
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
		Summary("User login").
		Tags("Authentication").
		RequestModel(internal.LoginRequest{}).
		ResponseModel(internal.LoginResponse{})

	// Protected: every route below needs "Authorization: Bearer <token>".
	api := app.Group("/api/v1", middleware.JWT(middleware.JWTConfig{Secret: internal.Secret}))

	api.GET("/todos", func(c *goichi.Context) error {
		mu.RLock()
		defer mu.RUnlock()
		list := make([]internal.Todo, 0, len(todos))
		for _, t := range todos {
			list = append(list, t)
		}
		return c.Ok(list)
	}).Summary("List todos").Tags("Todos").ResponseModel([]internal.Todo{})

	api.GET("/todos/:id", func(c *goichi.Context) error {
		mu.RLock()
		defer mu.RUnlock()
		t, ok := todos[c.ParamInt("id", 0)]
		if !ok {
			return c.NotFound("todo not found")
		}
		return c.Ok(t)
	}).Summary("Get one todo").Tags("Todos").ResponseModel(internal.Todo{})

	api.POST("/todos", func(c *goichi.Context) error {
		var t internal.Todo
		if err := c.Bind(&t); err != nil {
			return c.BadRequest(err.Error())
		}
		mu.Lock()
		defer mu.Unlock()
		t.ID = nextID
		nextID++
		todos[t.ID] = t
		return c.Created(t)
	}).Summary("Create a todo").Tags("Todos").RequestModel(internal.Todo{}).ResponseModel(internal.Todo{})

	api.DELETE("/todos/:id", func(c *goichi.Context) error {
		id := c.ParamInt("id", 0)
		mu.Lock()
		defer mu.Unlock()
		if _, ok := todos[id]; !ok {
			return c.NotFound("todo not found")
		}
		delete(todos, id)
		return c.NoContent()
	}).Summary("Delete a todo").Tags("Todos")

	// Shows who the token belongs to.
	api.GET("/me", func(c *goichi.Context) error {
		return c.Ok(c.JWT())
	}).Summary("Current token claims").Tags("Authentication")

	log.Println("REST example -> http://127.0.0.1:8080/docs")
	if err := app.ListenGraceful("127.0.0.1:8080"); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
