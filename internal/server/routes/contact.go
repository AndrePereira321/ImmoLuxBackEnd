package routes

import (
	"context"
	"fmt"

	"immo-lux/internal/database/ent/client"
	"immo-lux/internal/models"
	"immo-lux/internal/server_error"

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
		UserID: &userId,
		Name:   &payload.Name,
		Email:  &payload.Email,
		Phone:  &payload.Phone,
		Notes:  payload.Notes,
	}

	var contactId models.RecordId
	err = ctx.Db().WithTransaction(func(txCtx context.Context, tx *client.Tx) error {
		id, createErr := ctx.Db().NewContactRepository().CreateContact(txCtx, tx, contactDTO)
		if createErr != nil {
			return createErr
		}
		contactId = id
		return nil
	})
	if err != nil {
		ctx.Logger().Error(fmt.Sprintf("Failed to create contact: %s", err.Error()))
		return err
	}

	ctx.Logger().Info(fmt.Sprintf("Contact created by user %d: %d", userId, contactId))

	createdContact, err := ctx.Db().NewContactRepository().GetContactById(context.Background(), contactId)
	if err != nil {
		return err
	}

	return ctx.RespondData(createdContact)
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

	contact, err := ctx.Db().NewContactRepository().GetContactById(context.Background(), contactId)
	if err != nil {
		if server_error.IsServerError(err, "CONTACT_NOT_FOUND") {
			return ctx.RespondError(fiber.StatusNotFound, "CONTACT_NOT_FOUND", "Contact not found")
		}
		return err
	}

	if *contact.UserID != userId {
		return ctx.RespondError(fiber.StatusForbidden, "ACCESS_DENIED", "You can only view your own contacts")
	}

	return ctx.RespondData(contact)
}

func ListMyContacts(ctx *RouteContext) error {
	userId, err := ctx.RequireUserId()
	if err != nil {
		return err
	}

	contacts, err := ctx.Db().NewContactRepository().GetContactsByUserId(context.Background(), userId)
	if err != nil {
		ctx.Logger().Error(fmt.Sprintf("Failed to list contacts: %s", err.Error()))
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

	existingContact, err := ctx.Db().NewContactRepository().GetContactById(context.Background(), contactId)
	if err != nil {
		if server_error.IsServerError(err, "CONTACT_NOT_FOUND") {
			return ctx.RespondError(fiber.StatusNotFound, "CONTACT_NOT_FOUND", "Contact not found")
		}
		return err
	}

	if *existingContact.UserID != userId {
		return ctx.RespondError(fiber.StatusForbidden, "ACCESS_DENIED", "You can only update your own contacts")
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

	err = ctx.Db().WithTransaction(func(txCtx context.Context, tx *client.Tx) error {
		return ctx.Db().NewContactRepository().UpdateContact(txCtx, tx, contactId, contactDTO)
	})

	if err != nil {
		ctx.Logger().Error(fmt.Sprintf("Failed to update contact %d: %s", contactId, err.Error()))
		return err
	}

	ctx.Logger().Info(fmt.Sprintf("Contact updated by user %d: %d", userId, contactId))

	updatedContact, err := ctx.Db().NewContactRepository().GetContactById(context.Background(), contactId)
	if err != nil {
		return err
	}

	return ctx.RespondData(updatedContact)
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

	existingContact, err := ctx.Db().NewContactRepository().GetContactById(context.Background(), contactId)
	if err != nil {
		if server_error.IsServerError(err, "CONTACT_NOT_FOUND") {
			return ctx.RespondError(fiber.StatusNotFound, "CONTACT_NOT_FOUND", "Contact not found")
		}
		return err
	}

	if *existingContact.UserID != userId {
		return ctx.RespondError(fiber.StatusForbidden, "ACCESS_DENIED", "You can only delete your own contacts")
	}

	err = ctx.Db().WithTransaction(func(txCtx context.Context, tx *client.Tx) error {
		return ctx.Db().NewContactRepository().DeleteContact(txCtx, tx, contactId)
	})

	if err != nil {
		ctx.Logger().Error(fmt.Sprintf("Failed to delete contact %d: %s", contactId, err.Error()))
		return err
	}

	ctx.Logger().Info(fmt.Sprintf("Contact deleted by user %d: %d", userId, contactId))

	return ctx.RespondData(&fiber.Map{
		"success": true,
		"message": "Contact deleted successfully",
	})
}
