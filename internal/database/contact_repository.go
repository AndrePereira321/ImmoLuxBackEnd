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

// ensureAllOwned verifies every contact in ids exists and belongs to ownerId
// with one batched query, probing per-id only to name the offender.
func (rep *ContactRepository) ensureAllOwned(ctx context.Context, tx *client.Tx, ownerId models.RecordId, ids []models.RecordId) error {
	if len(ids) == 0 {
		return nil
	}

	seen := make(map[models.RecordId]bool, len(ids))
	intIds := make([]int, 0, len(ids))
	for _, id := range ids {
		if !id.IsValid() {
			return server_error.Invalid("INVALID_CONTACT_ID", "contact ID is invalid")
		}
		if !seen[id] {
			seen[id] = true
			intIds = append(intIds, int(id))
		}
	}

	ownedCount, err := tx.Contact.Query().
		Where(contact.IDIn(intIds...), contact.UserIDEQ(int(ownerId))).
		Count(ctx)
	if err != nil {
		return server_error.Wrap("CONTACT_QUERY", "failed to check contact ownership", err)
	}
	if ownedCount == len(intIds) {
		return nil
	}

	for _, id := range intIds {
		owned, err := tx.Contact.Query().
			Where(contact.IDEQ(id), contact.UserIDEQ(int(ownerId))).
			Exist(ctx)
		if err != nil {
			return server_error.Wrap("CONTACT_QUERY", "failed to check contact ownership", err)
		}
		if owned {
			continue
		}
		return ownershipVerdict(ctx,
			func(c context.Context) (bool, error) {
				return tx.Contact.Query().Where(contact.IDEQ(id)).Exist(c)
			},
			"CONTACT_QUERY",
			server_error.NotFound("CONTACT_NOT_FOUND", "contact not found"),
			server_error.Forbidden("CONTACT_ACCESS_DENIED", "you can only use your own contacts"),
		)
	}
	return nil
}

// CreateContact stores a new contact for ownerId and returns the stored record.
func (rep *ContactRepository) CreateContact(ctx context.Context, ownerId models.RecordId, contactDTO *models.ContactDTO) (*models.ContactDTO, error) {
	var created *client.Contact
	err := rep.db.WithTransaction(ctx, func(txCtx context.Context, tx *client.Tx) error {
		node, err := rep.createContact(txCtx, tx, ownerId, contactDTO)
		if err != nil {
			return err
		}
		created = node
		return nil
	})
	if err != nil {
		return nil, err
	}

	return rep.entToDTO(created), nil
}

// createContact inserts a contact for ownerId inside the caller's transaction;
// the property intake operation uses it when a new contact arrives inline.
func (rep *ContactRepository) createContact(ctx context.Context, tx *client.Tx, ownerId models.RecordId, contactDTO *models.ContactDTO) (*client.Contact, error) {
	contactDTO.UserID = &ownerId
	if err := rep.validateContactInput(contactDTO); err != nil {
		return nil, err
	}

	rep.db.Logger().Debug(fmt.Sprintf("Creating contact: %s", *contactDTO.Name))

	builder := tx.Contact.Create().
		SetUserID(int(ownerId)).
		SetName(*contactDTO.Name).
		SetEmail(*contactDTO.Email).
		SetPhone(*contactDTO.Phone).
		SetNillableNotes(contactDTO.Notes)

	createdContact, err := builder.Save(ctx)
	if err != nil {
		rep.db.Logger().Error(fmt.Sprintf("Failed to create contact [%s]: %s", *contactDTO.Name, err.Error()))
		return nil, server_error.Wrap("CONTACT_INSERT", "failed to insert contact", err)
	}

	rep.db.Logger().Info(fmt.Sprintf("Contact created successfully: %d", createdContact.ID))
	return createdContact, nil
}

// GetOwnedContact returns a contact ownerId owns, distinguishing a missing
// contact from someone else's.
func (rep *ContactRepository) GetOwnedContact(ctx context.Context, ownerId, contactId models.RecordId) (*models.ContactDTO, error) {
	if !contactId.IsValid() {
		return nil, server_error.Invalid("INVALID_CONTACT_ID", "contact ID is invalid")
	}

	cont, err := rep.db.client.Contact.Query().
		Where(contact.IDEQ(int(contactId)), contact.UserIDEQ(int(ownerId))).
		Only(ctx)
	if err == nil {
		return rep.entToDTO(cont), nil
	}
	if !client.IsNotFound(err) {
		return nil, server_error.Wrap("CONTACT_QUERY", "failed to query contact", err)
	}

	return nil, ownershipVerdict(ctx,
		func(c context.Context) (bool, error) {
			return rep.db.client.Contact.Query().Where(contact.IDEQ(int(contactId))).Exist(c)
		},
		"CONTACT_QUERY",
		server_error.NotFound("CONTACT_NOT_FOUND", "contact not found"),
		server_error.Forbidden("ACCESS_DENIED", "you can only access your own contacts"),
	)
}

func (rep *ContactRepository) GetContactsByUserId(ctx context.Context, userId models.RecordId) ([]models.ContactDTO, error) {
	if !userId.IsValid() {
		return nil, server_error.Invalid("INVALID_USER_ID", "user ID is invalid")
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

// UpdateContact applies the given field changes to a contact ownerId owns and
// returns the updated record.
func (rep *ContactRepository) UpdateContact(ctx context.Context, ownerId, contactId models.RecordId, contactDTO *models.ContactDTO) (*models.ContactDTO, error) {
	err := rep.db.WithTransaction(ctx, func(txCtx context.Context, tx *client.Tx) error {
		if err := rep.ensureAllOwned(txCtx, tx, ownerId, []models.RecordId{contactId}); err != nil {
			return err
		}

		rep.db.Logger().Debug(fmt.Sprintf("Updating contact: %d", contactId))

		builder := tx.Contact.UpdateOneID(int(contactId))

		if contactDTO.Name != nil {
			builder.SetName(*contactDTO.Name)
		}
		if contactDTO.Email != nil {
			if !utils.IsValidEmail(*contactDTO.Email) {
				return server_error.Invalid("CONTACT_VALIDATION", "email is invalid")
			}
			builder.SetEmail(*contactDTO.Email)
		}
		if contactDTO.Phone != nil {
			builder.SetPhone(*contactDTO.Phone)
		}
		builder.SetNillableNotes(contactDTO.Notes)

		if err := builder.Exec(txCtx); err != nil {
			rep.db.Logger().Error(fmt.Sprintf("Failed to update contact [%d]: %s", contactId, err.Error()))
			return server_error.Wrap("CONTACT_UPDATE", "failed to update contact", err)
		}

		rep.db.Logger().Info(fmt.Sprintf("Contact updated successfully: %d", contactId))
		return nil
	})
	if err != nil {
		return nil, err
	}

	return rep.GetOwnedContact(ctx, ownerId, contactId)
}

// DeleteContact removes a contact ownerId owns with one owner-predicated
// statement, probing only when nothing matched.
func (rep *ContactRepository) DeleteContact(ctx context.Context, ownerId, contactId models.RecordId) error {
	if !contactId.IsValid() {
		return server_error.Invalid("INVALID_CONTACT_ID", "contact ID is invalid")
	}

	deleted, err := rep.db.client.Contact.Delete().
		Where(contact.IDEQ(int(contactId)), contact.UserIDEQ(int(ownerId))).
		Exec(ctx)
	if err != nil {
		rep.db.Logger().Error(fmt.Sprintf("Failed to delete contact [%d]: %s", contactId, err.Error()))
		return server_error.Wrap("CONTACT_DELETE", "failed to delete contact", err)
	}
	if deleted == 0 {
		return ownershipVerdict(ctx,
			func(c context.Context) (bool, error) {
				return rep.db.client.Contact.Query().Where(contact.IDEQ(int(contactId))).Exist(c)
			},
			"CONTACT_QUERY",
			server_error.NotFound("CONTACT_NOT_FOUND", "contact not found"),
			server_error.Forbidden("ACCESS_DENIED", "you can only access your own contacts"),
		)
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
			return nil, server_error.NotFound("CONTACT_NOT_FOUND", "contact not found")
		}
		return nil, server_error.Wrap("CONTACT_QUERY", "failed to query contact", err)
	}

	return rep.entToDTO(cont), nil
}

func (rep *ContactRepository) validateContactInput(contactDTO *models.ContactDTO) error {
	if contactDTO.Name == nil || *contactDTO.Name == "" {
		return server_error.Invalid("CONTACT_VALIDATION", "name is required")
	}
	if contactDTO.Email == nil || *contactDTO.Email == "" {
		return server_error.Invalid("CONTACT_VALIDATION", "email is required")
	}
	if !utils.IsValidEmail(*contactDTO.Email) {
		return server_error.Invalid("CONTACT_VALIDATION", "email is invalid")
	}
	if contactDTO.Phone == nil || *contactDTO.Phone == "" {
		return server_error.Invalid("CONTACT_VALIDATION", "phone is required")
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
