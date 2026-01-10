package database

import (
	"context"
	"fmt"

	"immo-lux/internal/database/ent/client"
	"immo-lux/internal/database/ent/client/contact"
	"immo-lux/internal/models"
	"immo-lux/internal/server_error"
	"immo-lux/internal/utils"
)

type ContactRepository struct {
	db *Database
}

func NewContactRepository(db *Database) *ContactRepository {
	return &ContactRepository{db: db}
}

func (rep *ContactRepository) CreateContact(ctx context.Context, tx *client.Tx, contactDTO *models.ContactDTO) (models.RecordId, error) {
	if err := rep.validateContactInput(contactDTO); err != nil {
		return models.InvalidRecordId, err
	}

	rep.db.Logger().Debug(fmt.Sprintf("Creating contact: %s", *contactDTO.Name))

	builder := tx.Contact.Create().
		SetUserID(int(*contactDTO.UserID)).
		SetName(*contactDTO.Name).
		SetEmail(*contactDTO.Email).
		SetPhone(*contactDTO.Phone)

	if contactDTO.Notes != nil {
		builder.SetNotes(*contactDTO.Notes)
	}

	createdContact, err := builder.Save(ctx)
	if err != nil {
		rep.db.Logger().Error(fmt.Sprintf("Failed to create contact [%s]: %s", *contactDTO.Name, err.Error()))
		return models.InvalidRecordId, server_error.Wrap("CONTACT_INSERT", "failed to insert contact", err)
	}

	rep.db.Logger().Info(fmt.Sprintf("Contact created successfully: %d", createdContact.ID))
	return models.RecordId(createdContact.ID), nil
}

func (rep *ContactRepository) GetContactById(ctx context.Context, contactId models.RecordId) (*models.ContactDTO, error) {
	if !contactId.IsValid() {
		return nil, server_error.New("INVALID_CONTACT_ID", "contact ID is invalid")
	}

	cont, err := rep.db.client.Contact.Query().
		Where(contact.IDEQ(int(contactId))).
		Only(ctx)

	if err != nil {
		if client.IsNotFound(err) {
			return nil, server_error.New("CONTACT_NOT_FOUND", "contact not found")
		}
		return nil, server_error.Wrap("CONTACT_QUERY", "failed to query contact", err)
	}

	return rep.entToDTO(cont), nil
}

func (rep *ContactRepository) GetContactsByUserId(ctx context.Context, userId models.RecordId) ([]models.ContactDTO, error) {
	if !userId.IsValid() {
		return nil, server_error.New("INVALID_USER_ID", "user ID is invalid")
	}

	contacts, err := rep.db.client.Contact.Query().
		Where(contact.UserIDEQ(int(userId))).
		Order(client.Desc(contact.FieldCreatedAt)).
		All(ctx)

	if err != nil {
		return nil, server_error.Wrap("CONTACT_LIST", "failed to list contacts", err)
	}

	dtos := make([]models.ContactDTO, len(contacts))
	for i, cont := range contacts {
		dtos[i] = *rep.entToDTO(cont)
	}

	return dtos, nil
}

func (rep *ContactRepository) UpdateContact(ctx context.Context, tx *client.Tx, contactId models.RecordId, contactDTO *models.ContactDTO) error {
	if !contactId.IsValid() {
		return server_error.New("INVALID_CONTACT_ID", "contact ID is invalid")
	}

	rep.db.Logger().Debug(fmt.Sprintf("Updating contact: %d", contactId))

	builder := tx.Contact.UpdateOneID(int(contactId))

	if contactDTO.Name != nil {
		builder.SetName(*contactDTO.Name)
	}
	if contactDTO.Email != nil {
		if !utils.IsValidEmail(*contactDTO.Email) {
			return server_error.New("CONTACT_VALIDATION", "email is invalid")
		}
		builder.SetEmail(*contactDTO.Email)
	}
	if contactDTO.Phone != nil {
		builder.SetPhone(*contactDTO.Phone)
	}
	if contactDTO.Notes != nil {
		builder.SetNillableNotes(contactDTO.Notes)
	}

	err := builder.Exec(ctx)
	if err != nil {
		if client.IsNotFound(err) {
			return server_error.New("CONTACT_NOT_FOUND", "contact not found")
		}
		rep.db.Logger().Error(fmt.Sprintf("Failed to update contact [%d]: %s", contactId, err.Error()))
		return server_error.Wrap("CONTACT_UPDATE", "failed to update contact", err)
	}

	rep.db.Logger().Info(fmt.Sprintf("Contact updated successfully: %d", contactId))
	return nil
}

func (rep *ContactRepository) DeleteContact(ctx context.Context, tx *client.Tx, contactId models.RecordId) error {
	if !contactId.IsValid() {
		return server_error.New("INVALID_CONTACT_ID", "contact ID is invalid")
	}

	rep.db.Logger().Debug(fmt.Sprintf("Deleting contact: %d", contactId))

	err := tx.Contact.DeleteOneID(int(contactId)).Exec(ctx)
	if err != nil {
		if client.IsNotFound(err) {
			return server_error.New("CONTACT_NOT_FOUND", "contact not found")
		}
		rep.db.Logger().Error(fmt.Sprintf("Failed to delete contact [%d]: %s", contactId, err.Error()))
		return server_error.Wrap("CONTACT_DELETE", "failed to delete contact", err)
	}

	rep.db.Logger().Info(fmt.Sprintf("Contact deleted successfully: %d", contactId))
	return nil
}

func (rep *ContactRepository) FindByEmailAndUser(ctx context.Context, userId models.RecordId, email string) (*models.ContactDTO, error) {
	cont, err := rep.db.client.Contact.Query().
		Where(
			contact.UserIDEQ(int(userId)),
			contact.EmailEQ(email),
		).
		Only(ctx)

	if err != nil {
		if client.IsNotFound(err) {
			return nil, server_error.New("CONTACT_NOT_FOUND", "contact not found")
		}
		return nil, server_error.Wrap("CONTACT_QUERY", "failed to query contact", err)
	}

	return rep.entToDTO(cont), nil
}

func (rep *ContactRepository) validateContactInput(contactDTO *models.ContactDTO) error {
	if contactDTO.Name == nil || *contactDTO.Name == "" {
		return server_error.New("CONTACT_VALIDATION", "name is required")
	}
	if contactDTO.Email == nil || *contactDTO.Email == "" {
		return server_error.New("CONTACT_VALIDATION", "email is required")
	}
	if !utils.IsValidEmail(*contactDTO.Email) {
		return server_error.New("CONTACT_VALIDATION", "email is invalid")
	}
	if contactDTO.Phone == nil || *contactDTO.Phone == "" {
		return server_error.New("CONTACT_VALIDATION", "phone is required")
	}
	if contactDTO.UserID == nil || !contactDTO.UserID.IsValid() {
		return server_error.New("CONTACT_VALIDATION", "user ID is required")
	}

	return nil
}

func (rep *ContactRepository) entToDTO(cont *client.Contact) *models.ContactDTO {
	contactId := models.RecordId(cont.ID)
	userId := models.RecordId(cont.UserID)

	dto := &models.ContactDTO{
		ID:        &contactId,
		UserID:    &userId,
		Name:      &cont.Name,
		Email:     &cont.Email,
		Phone:     &cont.Phone,
		CreatedAt: &cont.CreatedAt,
		UpdatedAt: &cont.UpdatedAt,
	}

	if cont.Notes != nil {
		dto.Notes = cont.Notes
	}

	return dto
}
