package routes

import (
	"immo-lux/internal/models"

	"github.com/gofiber/fiber/v3"
)

type CreateContactPayload struct {
	Name  string  `json:"name"`
	Email string  `json:"email"`
	Phone string  `json:"phone"`
	Notes *string `json:"notes"`
}

type UpdateContactPayload struct {
	Name  *string `json:"name"`
	Email *string `json:"email"`
	Phone *string `json:"phone"`
	Notes *string `json:"notes"`
}

func CreateContact(ctx *RouteContext) error {
	payload := &CreateContactPayload{}
	if err := ctx.ReadBody(&payload); err != nil {
		return err
	}

	userId, err := ctx.RequireUserId()
	if err != nil {
		return err
	}

	contactDTO := &models.ContactDTO{
		Name:  &payload.Name,
		Email: &payload.Email,
		Phone: &payload.Phone,
		Notes: payload.Notes,
	}

	created, err := ctx.Db().NewContactRepository().CreateContact(ctx.RequestContext(), userId, contactDTO)
	if err != nil {
		return err
	}

	return ctx.RespondData(created)
}

func GetContact(ctx *RouteContext) error {
	contactId, err := ctx.ParseIdParam("id")
	if err != nil {
		return err
	}

	userId, err := ctx.RequireUserId()
	if err != nil {
		return err
	}

	contact, err := ctx.Db().NewContactRepository().GetOwnedContact(ctx.RequestContext(), userId, contactId)
	if err != nil {
		return err
	}

	return ctx.RespondData(contact)
}

func ListMyContacts(ctx *RouteContext) error {
	userId, err := ctx.RequireUserId()
	if err != nil {
		return err
	}

	contacts, err := ctx.Db().NewContactRepository().GetContactsByUserId(ctx.RequestContext(), userId)
	if err != nil {
		return err
	}

	return ctx.RespondData(&fiber.Map{
		"contacts": contacts,
	})
}

func UpdateContact(ctx *RouteContext) error {
	contactId, err := ctx.ParseIdParam("id")
	if err != nil {
		return err
	}

	userId, err := ctx.RequireUserId()
	if err != nil {
		return err
	}

	payload := &UpdateContactPayload{}
	if err := ctx.ReadBody(&payload); err != nil {
		return err
	}

	contactDTO := &models.ContactDTO{
		Name:  payload.Name,
		Email: payload.Email,
		Phone: payload.Phone,
		Notes: payload.Notes,
	}

	updated, err := ctx.Db().NewContactRepository().UpdateContact(ctx.RequestContext(), userId, contactId, contactDTO)
	if err != nil {
		return err
	}

	return ctx.RespondData(updated)
}

func DeleteContact(ctx *RouteContext) error {
	contactId, err := ctx.ParseIdParam("id")
	if err != nil {
		return err
	}

	userId, err := ctx.RequireUserId()
	if err != nil {
		return err
	}

	if err := ctx.Db().NewContactRepository().DeleteContact(ctx.RequestContext(), userId, contactId); err != nil {
		return err
	}

	return ctx.RespondData(&fiber.Map{
		"success": true,
		"message": "Contact deleted successfully",
	})
}
