package server

import (
	"immo-lux/internal/server/routes"
	"path/filepath"
	"strings"

	"github.com/gofiber/fiber/v3"
)

type RouteHandler func(ctx *routes.RouteContext) error

func (s *Server) RegisterRoutes() {
	s.Get("/ping", routes.Ping)
	s.Post("/login", routes.Login)

	s.serveSPA()
}

func (s *Server) serveSPA() {
	spaFolder := s.config.HttpServer().SpaFolder()
	if spaFolder == "" {
		return
	}

	mfs, err := newMemoryFS(spaFolder)
	if err != nil {
		panic("Failed to load SPA files into memory: " + err.Error())
	}

	s.fiber.Use(func(c fiber.Ctx) error {
		if strings.HasPrefix(c.Path(), ApiPrefix) {
			return c.Next()
		}

		path := c.Path()
		data, err := mfs.Open(path)
		if err == nil {
			ext := filepath.Ext(path)
			if ext != "" {
				c.Type(ext[1:])
			}
			return c.Send(data)
		}

		data, err = mfs.Open("/index.html")
		if err != nil {
			return c.Status(404).SendString("Not found")
		}
		c.Type("html")
		return c.Send(data)
	})
}
