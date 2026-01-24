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
		acceptEncoding := c.Get("Accept-Encoding")

		if tryServeFile(c, mfs, path, acceptEncoding) {
			return nil
		}

		if tryServeFile(c, mfs, "/index.html", acceptEncoding) {
			return nil
		}

		return c.Status(404).SendString("Not found")
	})
}

func tryServeFile(c fiber.Ctx, mfs *memoryFS, path string, acceptEncoding string) bool {
	ext := filepath.Ext(path)

	if strings.Contains(acceptEncoding, "br") && tryServeWithEncoding(c, mfs, path+".br", ext, "br") {
		return true
	}

	if strings.Contains(acceptEncoding, "gzip") && tryServeWithEncoding(c, mfs, path+".gz", ext, "gzip") {
		return true
	}

	return tryServeWithEncoding(c, mfs, path, ext, "")
}

func tryServeWithEncoding(c fiber.Ctx, mfs *memoryFS, path string, ext string, encoding string) bool {
	data, err := mfs.Open(path)
	if err != nil {
		return false
	}

	if encoding != "" {
		c.Set("Content-Encoding", encoding)
	}
	if ext != "" {
		c.Type(ext[1:])
	}
	c.Send(data)
	return true
}
