package server

import (
	"immo-lux/internal/server/routes"
)

type RouteHandler func(ctx *routes.RouteContext) error

func (s *Server) RegisterRoutes() {
	s.Get("/ping", routes.Ping)
	s.Post("/login", routes.Login)
	s.Get("/isconnected", routes.IsConnected)

	s.SecuredPost("/logout", routes.Logout)
	s.SecuredGet("/sessions", routes.GetSessions)
	s.SecuredPost("/sessions/revoke", routes.RevokeSession)

	s.Get("/locations", routes.GetLocations)

	s.Get("/properties", routes.ListProperties)
	s.Get("/properties/:id", routes.GetProperty)
	s.Get("/properties/:id/images", routes.GetPropertyImages)
	s.Get("/images/:id", routes.GetImage)

	s.SecuredGet("/my-properties", routes.ListMyProperties)
	s.SecuredPost("/properties", routes.CreateProperty)
	s.SecuredPut("/properties/:id", routes.UpdateProperty)
	s.SecuredDelete("/properties/:id", routes.DeleteProperty)
	s.SecuredPost("/properties/:id/publish", routes.PublishProperty)
	s.SecuredPost("/properties/:id/unpublish", routes.UnpublishProperty)
	s.SecuredPost("/properties/:id/images", routes.UploadPropertyImage)
	s.SecuredPut("/images/:id/order", routes.UpdatePropertyImageOrder)
	s.SecuredDelete("/images/:id", routes.DeletePropertyImage)

	s.SecuredGet("/contacts", routes.ListMyContacts)
	s.SecuredPost("/contacts", routes.CreateContact)
	s.SecuredGet("/contacts/:id", routes.GetContact)
	s.SecuredPut("/contacts/:id", routes.UpdateContact)
	s.SecuredDelete("/contacts/:id", routes.DeleteContact)
}
