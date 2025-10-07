package server

import "immo-lux/internal/server/routes"

type RouteHandler func(ctx *routes.RouteContext) error

func (s *Server) RegisterRoutes() {
	s.Get("/ping", routes.Ping)
	s.Post("/login", routes.Login)
}
