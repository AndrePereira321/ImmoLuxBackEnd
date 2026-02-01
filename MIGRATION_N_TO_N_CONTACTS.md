# Migration Guide: Property-Contact N:1 → N:N Relationship

## Overview

This document outlines the complete migration from a **Many-to-One** (N:1) relationship between Property and Contact to a **Many-to-Many** (N:N) relationship, allowing multiple contacts per property.

## Changes Summary

### Database Schema Changes
- ✅ Removed `contact_id` field from Property table
- ✅ Changed Property → Contact edge from `Unique()` to allow multiple
- ✅ Ent will automatically create a join table: `property_contacts`

### Code Changes Required
1. ✅ Update Ent schemas (DONE)
2. ✅ Update PropertyDTO model (DONE)
3. ⚠️ Generate Ent code (YOU MUST RUN)
4. ⚠️ Update property_repository.go (SEE BELOW)
5. ⚠️ Update property_user.go routes (SEE BELOW)
6. ⚠️ Update property_public.go routes (SEE BELOW)
7. ⚠️ Update documentation (SEE BELOW)

---

## Step 1: Generate Ent Code (REQUIRED)

Run this command in your project root:

```bash
go generate ./internal/database/ent
```

This will generate:
- New join table methods
- Updated `AddContacts()`, `RemoveContacts()`, `ClearContacts()` methods
- Updated query methods with `WithContacts()`

---

## Step 2: Update property_repository.go

### File: `internal/database/property_repository.go`

#### A. Update `CreateProperty` method

**OLD CODE (lines ~38-56):**
```go
builder := tx.Property.Create().
    SetTitle(*propertyDTO.Title).
    // ... other fields ...
    SetContactID(int(*propertyDTO.ContactID)).
    SetPublisherID(int(*propertyDTO.PublisherID))
```

**NEW CODE:**
```go
builder := tx.Property.Create().
    SetTitle(*propertyDTO.Title).
    // ... other fields ...
    SetPublisherID(int(*propertyDTO.PublisherID))

// Add contacts if provided
if len(propertyDTO.ContactIDs) > 0 {
    contactIDs := make([]int, len(propertyDTO.ContactIDs))
    for i, cid := range propertyDTO.ContactIDs {
        contactIDs[i] = int(cid)
    }
    builder.AddContactIDs(contactIDs...)
}
```

#### B. Update `UpdateProperty` method

**Add after line ~240 (after setting ContactID):**
```go
// Handle contacts update (replace existing contacts)
if propertyDTO.ContactIDs != nil {
    // Clear existing contacts and add new ones
    update := tx.Property.UpdateOneID(int(propertyId)).
        ClearContacts()

    if len(propertyDTO.ContactIDs) > 0 {
        contactIDs := make([]int, len(propertyDTO.ContactIDs))
        for i, cid := range propertyDTO.ContactIDs {
            contactIDs[i] = int(cid)
        }
        update = update.AddContactIDs(contactIDs...)
    }

    err := update.Exec(ctx)
    if err != nil {
        return server_error.Wrap("PROPERTY_UPDATE_CONTACTS", "failed to update property contacts", err)
    }
}
```

#### C. Update `validatePropertyInput` method

**OLD CODE (lines ~447-449):**
```go
if propertyDTO.ContactID == nil || !propertyDTO.ContactID.IsValid() {
    return server_error.New("PROPERTY_VALIDATION", "contact ID is required")
}
```

**NEW CODE:**
```go
if len(propertyDTO.ContactIDs) == 0 {
    return server_error.New("PROPERTY_VALIDATION", "at least one contact is required")
}
```

#### D. Update `entToDTO` method

**OLD CODE (lines ~488-510):**
```go
contactId := models.RecordId(prop.ContactID)
// ...
dto := &models.PropertyDTO{
    // ...
    ContactID:    &contactId,
    // ...
}
```

**NEW CODE:**
```go
dto := &models.PropertyDTO{
    ID:           &recordId,
    // ... all other fields ...
    PublisherID:  &publisherId,
    // Remove ContactID field
    ViewCount:    &prop.ViewCount,
    CreatedAt:    &prop.CreatedAt,
    UpdatedAt:    &prop.UpdatedAt,
}

// Note: Contacts will be loaded separately when needed
```

#### E. Update `ListProperties` method

**Add after fetching properties (around line ~408):**
```go
properties, err := query.
    Limit(filters.Limit).
    Offset(filters.Offset).
    WithContacts().  // ADD THIS LINE - eager load contacts
    All(ctx)
```

#### F. Update `GetPropertyById` method

**OLD CODE (line ~131-133):**
```go
prop, err := rep.db.client.Property.Query().
    Where(property.IDEQ(int(propertyId))).
    Only(ctx)
```

**NEW CODE:**
```go
prop, err := rep.db.client.Property.Query().
    Where(property.IDEQ(int(propertyId))).
    WithContacts().  // Eager load contacts
    Only(ctx)
```

#### G. Add helper method to convert contacts

**Add this new method at the end of the file:**
```go
func (rep *PropertyRepository) LoadPropertyContacts(ctx context.Context, propertyDTO *models.PropertyDTO, entProperty *client.Property) error {
    if entProperty.Edges.Contacts == nil {
        return nil
    }

    contactDTOs := make([]models.ContactDTO, 0, len(entProperty.Edges.Contacts))
    contactIDs := make([]models.RecordId, 0, len(entProperty.Edges.Contacts))

    for _, contact := range entProperty.Edges.Contacts {
        contactId := models.RecordId(contact.ID)
        contactIDs = append(contactIDs, contactId)

        contactDTOs = append(contactDTOs, models.ContactDTO{
            ID:        &contactId,
            Name:      &contact.Name,
            Email:     &contact.Email,
            Phone:     &contact.Phone,
            Notes:     contact.Notes,
            CreatedAt: &contact.CreatedAt,
            UpdatedAt: &contact.UpdatedAt,
        })
    }

    propertyDTO.ContactIDs = contactIDs
    propertyDTO.Contacts = contactDTOs

    return nil
}
```

---

## Step 3: Update property_user.go

### File: `internal/server/routes/property_user.go`

#### A. Update `CreatePropertyPayload`

**OLD CODE (line ~52):**
```go
type CreatePropertyPayload struct {
    // ... fields ...
    Contact        *ContactPayload `json:"contact"`
}
```

**NEW CODE:**
```go
type CreatePropertyPayload struct {
    // ... fields ...
    Contacts       []*ContactPayload `json:"contacts"`  // Changed to array
}
```

#### B. Update `CreateProperty` handler

**OLD CODE (lines ~98-135):**
```go
if payload.Contact == nil {
    return ctx.BadRequest("Contact information is required")
}

var contactId models.RecordId

// ... single contact logic ...

contactId = models.RecordId(*payload.Contact.ID)
```

**NEW CODE:**
```go
if len(payload.Contacts) == 0 {
    return ctx.BadRequest("At least one contact is required")
}

var contactIds []models.RecordId

err := ctx.Db().WithTransaction(func(txCtx context.Context, tx *client.Tx) error {
    // Process each contact
    for _, contactPayload := range payload.Contacts {
        var cid models.RecordId

        if contactPayload.ID != nil {
            // Link existing contact
            cid = models.RecordId(*contactPayload.ID)

            existingContact, err := ctx.Db().NewContactRepository().GetContactById(txCtx, cid)
            if err != nil {
                return err
            }

            if *existingContact.UserID != userId {
                return server_error.New("CONTACT_ACCESS_DENIED", "You can only use your own contacts")
            }
        } else {
            // Create new contact
            if contactPayload.Name == nil || contactPayload.Email == nil || contactPayload.Phone == nil {
                return server_error.New("CONTACT_VALIDATION", "Contact name, email, and phone are required")
            }

            contactDTO := &models.ContactDTO{
                UserID: &userId,
                Name:   contactPayload.Name,
                Email:  contactPayload.Email,
                Phone:  contactPayload.Phone,
                Notes:  contactPayload.Notes,
            }

            id, err := ctx.Db().NewContactRepository().CreateContact(txCtx, tx, contactDTO)
            if err != nil {
                return err
            }
            cid = id
        }

        contactIds = append(contactIds, cid)
    }

    // Create property with all contacts
    propertyDTO := &models.PropertyDTO{
        // ... all fields ...
        ContactIDs:     contactIds,  // Changed from ContactID
        PublisherID:    &userId,
    }

    id, err := ctx.Db().NewPropertyRepository().CreateProperty(txCtx, tx, propertyDTO)
    if err != nil {
        return err
    }
    propertyId = id
    return nil
})
```

#### C. Update response loading

**OLD CODE (lines ~191-194):**
```go
contact, err := ctx.Db().NewContactRepository().GetContactById(context.Background(), contactId)
if err == nil {
    createdProperty.Contact = contact
}
```

**NEW CODE:**
```go
// Contacts are already loaded via WithContacts() in GetPropertyById
// Or manually load:
entProp, _ := ctx.Db().Client().Property.Query().
    Where(property.IDEQ(int(propertyId))).
    WithContacts().
    Only(context.Background())

if entProp != nil {
    ctx.Db().NewPropertyRepository().LoadPropertyContacts(context.Background(), createdProperty, entProp)
}
```

#### D. Update `ListMyProperties` handler

**OLD CODE (lines ~234-241):**
```go
for i := range properties {
    if properties[i].ContactID != nil {
        contact, err := ctx.Db().NewContactRepository().GetContactById(context.Background(), *properties[i].ContactID)
        if err == nil {
            properties[i].Contact = contact
        }
    }
}
```

**NEW CODE:**
```go
// Contacts are already loaded via WithContacts() in ListProperties
// No additional loading needed if WithContacts() is used in the query
```

#### E. Update `UpdatePropertyPayload`

**Add new field:**
```go
type UpdatePropertyPayload struct {
    // ... existing fields ...
    ContactIDs     *[]int64 `json:"contactIds"`  // New field for updating contacts
}
```

---

## Step 4: Update property_public.go

### File: `internal/server/routes/property_public.go`

Update similar to property_user.go - remove single contact loading logic since contacts are loaded via `WithContacts()`.

---

## Step 5: Database Migration

After generating Ent code, Ent will automatically handle the migration when you run the application. The migration will:

1. Create a new join table: `property_contacts`
2. Migrate existing `contact_id` data to the join table
3. Remove the `contact_id` column from `property` table

**Migration Table Structure:**
```sql
CREATE TABLE property_contacts (
    property_id INTEGER NOT NULL,
    contact_id INTEGER NOT NULL,
    PRIMARY KEY (property_id, contact_id),
    FOREIGN KEY (property_id) REFERENCES properties(id) ON DELETE CASCADE,
    FOREIGN KEY (contact_id) REFERENCES contacts(id) ON DELETE CASCADE
);
```

---

## Step 6: API Changes Documentation

### Request Format Changes

**OLD - Single Contact:**
```json
{
  "title": "Villa",
  "contact": {
    "id": 123
  }
}
```

**NEW - Multiple Contacts:**
```json
{
  "title": "Villa",
  "contacts": [
    { "id": 123 },
    { "id": 456 }
  ]
}
```

**OR Create new contacts:**
```json
{
  "title": "Villa",
  "contacts": [
    {
      "name": "John Doe",
      "email": "john@example.com",
      "phone": "+351 123456789"
    },
    {
      "name": "Jane Smith",
      "email": "jane@example.com",
      "phone": "+351 987654321"
    }
  ]
}
```

### Response Format Changes

**OLD:**
```json
{
  "id": 1,
  "title": "Villa",
  "contactId": 123,
  "contact": {
    "id": 123,
    "name": "John Doe"
  }
}
```

**NEW:**
```json
{
  "id": 1,
  "title": "Villa",
  "contactIds": [123, 456],
  "contacts": [
    {
      "id": 123,
      "name": "John Doe",
      "email": "john@example.com",
      "phone": "+351 123456789"
    },
    {
      "id": 456,
      "name": "Jane Smith",
      "email": "jane@example.com",
      "phone": "+351 987654321"
    }
  ]
}
```

---

## Step 7: Frontend Changes

### TypeScript Interface Update

```typescript
// OLD
interface PropertyDTO {
    contactId?: number;
    contact?: ContactDTO;
}

// NEW
interface PropertyDTO {
    contactIds?: number[];
    contacts?: ContactDTO[];
}
```

### Frontend Form Changes

**Property creation form should allow selecting multiple contacts:**

```svelte
{#if useExistingContacts}
  <div class="contact-selector">
    {#each contacts as contact}
      <label>
        <input
          type="checkbox"
          bind:group={selectedContactIds}
          value={contact.id}
        />
        {contact.name} - {contact.email}
      </label>
    {/each}
  </div>
{:else}
  <!-- Allow adding multiple new contacts -->
  {#each newContacts as newContact, i}
    <div class="contact-form">
      <input type="text" bind:value={newContact.name} placeholder="Name" />
      <input type="email" bind:value={newContact.email} placeholder="Email" />
      <input type="tel" bind:value={newContact.phone} placeholder="Phone" />
      <button onclick={() => removeContact(i)}>Remove</button>
    </div>
  {/each}
  <button onclick={addContactForm}>Add Another Contact</button>
{/if}
```

---

## Step 8: Testing Checklist

After migration:

- [ ] Run `go generate ./internal/database/ent`
- [ ] Compile the project: `go build ./internal`
- [ ] Run database migration (automatic on first startup)
- [ ] Test creating property with single contact
- [ ] Test creating property with multiple contacts
- [ ] Test creating property with mix of existing and new contacts
- [ ] Test updating property contacts (add/remove)
- [ ] Test listing properties (verify contacts loaded)
- [ ] Test GET /my-properties (verify contacts included)
- [ ] Test GET /properties/:id (verify all contacts returned)
- [ ] Verify cannot delete contact still linked to properties
- [ ] Test property deletion (verify join table entries removed)

---

## Rollback Plan

If issues occur:

1. Restore database backup
2. Revert schema changes in `property.go`
3. Restore `contact_id` field
4. Change edge back to `Unique()`
5. Run `go generate ./internal/database/ent`
6. Revert code changes

---

## Summary

This migration changes the Property-Contact relationship from **1 contact per property** to **multiple contacts per property**. Key benefits:

✅ **Flexibility**: Properties can have multiple contact persons (e.g., owner, agent, property manager)
✅ **Reusability**: Same contact can be linked to multiple properties
✅ **Backward compatible**: Can still use single contact if needed
✅ **Automatic migration**: Ent handles database schema changes

**IMPORTANT**: Remember to update your frontend to handle arrays of contacts instead of single contact objects!
