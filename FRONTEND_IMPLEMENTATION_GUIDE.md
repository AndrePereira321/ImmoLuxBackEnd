# Frontend Implementation Guide - N:N Property-Contact Relationship

## Overview

The backend has been updated to support **Many-to-Many (N:N)** relationships between Properties and Contacts. Each property can now have **multiple contacts** and each contact can be associated with **multiple properties**.

## Key Changes Summary

### Before (N:1 Relationship)
- Property had ONE contact via `contactId` field
- Property payload: `{ "contactId": 123 }` or `{ "contact": { "name": "..." } }`
- Response: `{ "contactId": 123, "contact": { ...contactObject } }`

### After (N:N Relationship)
- Property has MULTIPLE contacts via `contactIds` array
- Property payload: `{ "contactIds": [123, 456] }` or `{ "contacts": [{ "name": "..." }, ...] }`
- Response: `{ "contactIds": [123, 456], "contacts": [{...}, {...}] }`

---

## TypeScript Interface Updates

### Update PropertyDTO Interface

**File**: `src/lib/types/property.ts` (or wherever you define types)

```typescript
interface PropertyDTO {
    id?: number;
    title?: string;
    description?: string;
    propertyType?: 'house' | 'apartment' | 'villa' | 'townhouse' | 'land' | 'commercial';
    price?: number;
    status?: 'available' | 'pending' | 'sold' | 'rented';
    isPublished?: boolean;

    // Location
    address?: string;
    district?: string;      // Required on create
    municipality?: string;  // Required on create
    parish?: string;
    postalCode?: string;
    country?: string;
    latitude?: number;
    longitude?: number;

    // Property details
    bedrooms?: number;
    bathrooms?: number;
    areaSqm?: number;
    landAreaSqm?: number;
    yearBuilt?: number;
    floor?: number;
    totalFloors?: number;
    parkingSpaces?: number;

    // Features
    hasGarage?: boolean;
    hasGarden?: boolean;
    hasPool?: boolean;
    hasElevator?: boolean;
    energyRating?: 'Aplus' | 'A' | 'B' | 'C' | 'D' | 'E' | 'F' | 'G';
    virtualTourUrl?: string;

    // ⚠️ CHANGED: Now arrays instead of single values
    contactIds?: number[];     // Array of contact IDs
    contacts?: ContactDTO[];   // Array of contact objects

    publisherId?: number;
    viewCount?: number;
    publishedAt?: string;
    createdAt?: string;
    updatedAt?: string;
}

interface ContactDTO {
    id?: number;
    userId?: number;
    name?: string;
    email?: string;
    phone?: string;
    notes?: string;
    createdAt?: string;
    updatedAt?: string;
}
```

---

## API Request Format Changes

### Creating a Property with Existing Contacts

**Old Format** (N:1):
```json
{
  "title": "Beautiful Villa",
  "...": "...",
  "contact": { "id": 123 }
}
```

**New Format** (N:N):
```json
{
  "title": "Beautiful Villa",
  "...": "...",
  "contactIds": [123, 456]
}
```

### Creating a Property with New Contacts

**Old Format** (N:1):
```json
{
  "title": "Beautiful Villa",
  "...": "...",
  "contact": {
    "name": "John Doe",
    "email": "john@example.com",
    "phone": "+351 123456789"
  }
}
```

**New Format** (N:N):
```json
{
  "title": "Beautiful Villa",
  "...": "...",
  "contacts": [
    {
      "name": "John Doe (Owner)",
      "email": "john@example.com",
      "phone": "+351 123456789",
      "notes": "Property owner"
    },
    {
      "name": "Maria Silva (Agent)",
      "email": "maria@agency.com",
      "phone": "+351 987654321",
      "notes": "Real estate agent"
    }
  ]
}
```

### Mix of Existing and New Contacts

```json
{
  "title": "Beautiful Villa",
  "...": "...",
  "contacts": [
    { "id": 123 },  // Existing contact
    {               // New contact
      "name": "New Agent",
      "email": "agent@example.com",
      "phone": "+351 555555555"
    }
  ]
}
```

---

## Frontend Component Changes

### 1. Property Creation Form

#### Update State Management

**Old (Svelte 5 with runes)**:
```typescript
let useExistingContact = $state(true);
let selectedContactId = $state<number | null>(null);
let newContact = $state({
    name: '',
    email: '',
    phone: '',
    notes: ''
});
```

**New (Svelte 5 with runes)**:
```typescript
let useExistingContacts = $state(true);
let selectedContactIds = $state<number[]>([]);
let newContacts = $state<Array<{
    name: string;
    email: string;
    phone: string;
    notes: string;
}>>([]);

// Helper to add a new contact form
const addNewContactForm = () => {
    newContacts = [...newContacts, { name: '', email: '', phone: '', notes: '' }];
};

// Helper to remove a contact form
const removeNewContactForm = (index: number) => {
    newContacts = newContacts.filter((_, i) => i !== index);
};
```

#### Update Form UI

**Old UI**:
```svelte
<div>
    <label>
        <input type="radio" bind:group={useExistingContact} value={true} />
        Use Existing Contact
    </label>
    <label>
        <input type="radio" bind:group={useExistingContact} value={false} />
        Create New Contact
    </label>
</div>

{#if useExistingContact}
    <select bind:value={selectedContactId} required>
        <option value="">Select Contact...</option>
        {#each contacts as contact}
            <option value={contact.id}>{contact.name} - {contact.email}</option>
        {/each}
    </select>
{:else}
    <input type="text" bind:value={newContact.name} placeholder="Name" required />
    <input type="email" bind:value={newContact.email} placeholder="Email" required />
    <input type="tel" bind:value={newContact.phone} placeholder="Phone" required />
    <textarea bind:value={newContact.notes} placeholder="Notes" />
{/if}
```

**New UI (Multiple Contact Selection)**:
```svelte
<div class="contact-selection">
    <h3>Property Contacts</h3>

    <!-- Existing Contacts Multi-Select -->
    <div class="existing-contacts">
        <h4>Select Existing Contacts</h4>
        <div class="checkbox-list">
            {#each contacts as contact}
                <label class="contact-checkbox">
                    <input
                        type="checkbox"
                        value={contact.id}
                        checked={selectedContactIds.includes(contact.id!)}
                        onchange={(e) => {
                            if (e.currentTarget.checked) {
                                selectedContactIds = [...selectedContactIds, contact.id!];
                            } else {
                                selectedContactIds = selectedContactIds.filter(id => id !== contact.id);
                            }
                        }}
                    />
                    <span>{contact.name} - {contact.email} - {contact.phone}</span>
                </label>
            {/each}
        </div>
    </div>

    <!-- New Contacts Section -->
    <div class="new-contacts">
        <h4>Or Create New Contacts</h4>
        {#each newContacts as newContact, index}
            <div class="contact-form">
                <input
                    type="text"
                    bind:value={newContact.name}
                    placeholder="Name"
                    required
                />
                <input
                    type="email"
                    bind:value={newContact.email}
                    placeholder="Email"
                    required
                />
                <input
                    type="tel"
                    bind:value={newContact.phone}
                    placeholder="Phone"
                    required
                />
                <textarea
                    bind:value={newContact.notes}
                    placeholder="Notes (e.g., 'Property Owner', 'Real Estate Agent')"
                />
                <button
                    type="button"
                    onclick={() => removeNewContactForm(index)}
                >
                    Remove
                </button>
            </div>
        {/each}
        <button type="button" onclick={addNewContactForm}>
            + Add Another Contact
        </button>
    </div>

    <!-- Validation Message -->
    {#if selectedContactIds.length === 0 && newContacts.length === 0}
        <p class="error">At least one contact is required</p>
    {/if}
</div>
```

#### Update Form Submission

**Old**:
```typescript
const handleSubmit = async (e: Event) => {
    e.preventDefault();

    const payload = {
        ...property,
        contact: useExistingContact
            ? { id: selectedContactId }
            : newContact
    };

    const response = await apiClient.post<PropertyDTO>('/properties', payload);

    if (response.data.success) {
        // Success handling
    }
};
```

**New**:
```typescript
const handleSubmit = async (e: Event) => {
    e.preventDefault();

    // Validate at least one contact
    if (selectedContactIds.length === 0 && newContacts.length === 0) {
        alert('Please select at least one existing contact or create a new one');
        return;
    }

    // Prepare contacts payload
    let contactsPayload: Array<{id?: number} | ContactDTO> = [];

    // Add existing contacts
    if (selectedContactIds.length > 0) {
        contactsPayload = selectedContactIds.map(id => ({ id }));
    }

    // Add new contacts
    if (newContacts.length > 0) {
        const validNewContacts = newContacts.filter(c => c.name && c.email && c.phone);
        contactsPayload = [...contactsPayload, ...validNewContacts];
    }

    const payload = {
        ...property,
        contacts: contactsPayload  // ⚠️ Changed from 'contact' to 'contacts' array
    };

    const response = await apiClient.post<PropertyDTO>('/properties', payload);

    if (response.data.success) {
        const propertyId = response.data.data.id;

        // Upload images if any
        if (selectedImages.length > 0) {
            await uploadImages(propertyId);
        }

        goto(`/properties/${propertyId}`);
    } else {
        // Handle errors
        errorMessage = response.data.error?.message;
    }
};
```

### 2. Property Display Components

#### Property Card/List Item

**Old**:
```svelte
<div class="property-card">
    <h3>{property.title}</h3>
    <p>{property.price.toLocaleString('pt-PT')} €</p>
    <p>{property.district}, {property.municipality}</p>

    <!-- Single contact -->
    {#if property.contact}
        <p>Contact: {property.contact.name} - {property.contact.phone}</p>
    {/if}
</div>
```

**New**:
```svelte
<div class="property-card">
    <h3>{property.title}</h3>
    <p>{property.price.toLocaleString('pt-PT')} €</p>
    <p>{property.district}, {property.municipality}</p>

    <!-- Multiple contacts -->
    {#if property.contacts && property.contacts.length > 0}
        <div class="contacts">
            <p>Contacts:</p>
            <ul>
                {#each property.contacts as contact}
                    <li>{contact.name} - {contact.phone} - {contact.email}</li>
                {/each}
            </ul>
        </div>
    {/if}
</div>
```

#### Property Detail Page

**Old**:
```svelte
<div class="property-details">
    <h1>{property.title}</h1>
    <!-- ... other fields ... -->

    <section class="contact-section">
        <h2>Contact Information</h2>
        {#if property.contact}
            <p><strong>Name:</strong> {property.contact.name}</p>
            <p><strong>Email:</strong> {property.contact.email}</p>
            <p><strong>Phone:</strong> {property.contact.phone}</p>
            {#if property.contact.notes}
                <p><strong>Notes:</strong> {property.contact.notes}</p>
            {/if}
        {/if}
    </section>
</div>
```

**New**:
```svelte
<div class="property-details">
    <h1>{property.title}</h1>
    <!-- ... other fields ... -->

    <section class="contacts-section">
        <h2>Contact Information</h2>
        {#if property.contacts && property.contacts.length > 0}
            <div class="contacts-list">
                {#each property.contacts as contact, index}
                    <div class="contact-card">
                        <h3>Contact {index + 1}</h3>
                        <p><strong>Name:</strong> {contact.name}</p>
                        <p><strong>Email:</strong>
                            <a href="mailto:{contact.email}">{contact.email}</a>
                        </p>
                        <p><strong>Phone:</strong>
                            <a href="tel:{contact.phone}">{contact.phone}</a>
                        </p>
                        {#if contact.notes}
                            <p><strong>Role:</strong> {contact.notes}</p>
                        {/if}
                    </div>
                {/each}
            </div>
        {:else}
            <p>No contact information available</p>
        {/if}
    </section>
</div>
```

### 3. Property Edit Form

When editing a property, you'll need to load the existing contacts and allow modification:

```typescript
let property = $state<PropertyDTO | null>(null);
let selectedContactIds = $state<number[]>([]);
let newContacts = $state<Array<ContactDTO>>([]);

// Load property data
const loadProperty = async (id: number) => {
    const response = await apiClient.get<PropertyDTO>(`/properties/${id}`);
    if (response.data.success) {
        property = response.data.data;

        // Initialize selected contacts from property
        if (property.contactIds) {
            selectedContactIds = [...property.contactIds];
        }
    }
};

// Update property
const handleUpdate = async () => {
    // Prepare contacts payload (same as create)
    let contactsPayload: Array<{id?: number} | ContactDTO> = [];

    if (selectedContactIds.length > 0) {
        contactsPayload = selectedContactIds.map(id => ({ id }));
    }

    if (newContacts.length > 0) {
        const validNewContacts = newContacts.filter(c => c.name && c.email && c.phone);
        contactsPayload = [...contactsPayload, ...validNewContacts];
    }

    const payload = {
        ...property,
        contacts: contactsPayload
    };

    const response = await apiClient.put<PropertyDTO>(
        `/properties/${property.id}`,
        payload
    );

    if (response.data.success) {
        // Success handling
        goto(`/properties/${property.id}`);
    }
};
```

---

## Backend API Response Examples

### GET /properties/:id Response

```json
{
  "success": true,
  "data": {
    "id": 1,
    "title": "Beautiful Villa in Sintra",
    "price": 450000,
    "status": "available",
    "district": "Lisboa",
    "municipality": "Sintra",
    "contactIds": [123, 456],
    "contacts": [
      {
        "id": 123,
        "userId": 1,
        "name": "John Doe (Owner)",
        "email": "john@example.com",
        "phone": "+351 123456789",
        "notes": "Property owner - available weekdays",
        "createdAt": "2025-01-10T12:00:00Z",
        "updatedAt": "2025-01-10T12:00:00Z"
      },
      {
        "id": 456,
        "userId": 1,
        "name": "Maria Silva (Agent)",
        "email": "maria@agency.com",
        "phone": "+351 987654321",
        "notes": "Real estate agent - available anytime",
        "createdAt": "2025-01-15T10:00:00Z",
        "updatedAt": "2025-01-15T10:00:00Z"
      }
    ],
    "publisherId": 1,
    "...": "..."
  },
  "error": null
}
```

### GET /properties Response (List)

```json
{
  "success": true,
  "data": {
    "properties": [
      {
        "id": 1,
        "title": "Beautiful Villa",
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
            "name": "Maria Silva",
            "email": "maria@agency.com",
            "phone": "+351 987654321"
          }
        ],
        "...": "..."
      }
    ],
    "total": 1
  },
  "error": null
}
```

---

## Validation Rules

### Backend Validation

1. **At least one contact required**: Property must have at least one contact (either existing or new)
2. **Contact ownership**: All existing contacts must belong to the authenticated user
3. **New contact validation**: Each new contact must have:
   - `name` (max 150 chars)
   - `email` (valid email format, max 255 chars)
   - `phone` (max 20 chars)
   - `notes` (optional)

### Frontend Validation

```typescript
const validatePropertyForm = (): boolean => {
    // Check at least one contact
    if (selectedContactIds.length === 0 && newContacts.length === 0) {
        alert('At least one contact is required');
        return false;
    }

    // Validate new contacts
    for (const contact of newContacts) {
        if (!contact.name || contact.name.trim() === '') {
            alert('Contact name is required');
            return false;
        }
        if (!contact.email || !isValidEmail(contact.email)) {
            alert('Valid contact email is required');
            return false;
        }
        if (!contact.phone || contact.phone.trim() === '') {
            alert('Contact phone is required');
            return false;
        }
    }

    return true;
};
```

---

## Error Handling

### New Error Codes

- `PROPERTY_VALIDATION`: Missing or invalid required field (including contacts)
- `CONTACT_ACCESS_DENIED`: User trying to use another user's contact
- `CONTACT_NOT_FOUND`: Contact ID doesn't exist

### Example Error Handling

```typescript
const handlePropertyError = (error: ServerAPIError) => {
    const errorMessages: Record<string, string> = {
        'PROPERTY_VALIDATION': 'Please ensure all required fields are filled, including at least one contact',
        'CONTACT_ACCESS_DENIED': 'You can only use your own contacts',
        'CONTACT_NOT_FOUND': 'One or more selected contacts not found',
        // ... other errors
    };

    return errorMessages[error.code || ''] || error.message || 'An error occurred';
};
```

---

## Migration Checklist

### Required Changes

- [ ] Update TypeScript interfaces (`PropertyDTO`, change `contactId` → `contactIds`, `contact` → `contacts`)
- [ ] Update property creation form:
  - [ ] Add multiple contact selection UI
  - [ ] Update state management for arrays
  - [ ] Add "Add Contact" button functionality
  - [ ] Update submission payload format
- [ ] Update property edit form:
  - [ ] Load existing contacts as array
  - [ ] Allow adding/removing contacts
  - [ ] Update submission payload format
- [ ] Update property display components:
  - [ ] Property card - show multiple contacts
  - [ ] Property detail page - list all contacts
  - [ ] Property list - format multiple contacts
- [ ] Update form validation:
  - [ ] Check at least one contact
  - [ ] Validate each new contact
- [ ] Update error handling for new error codes
- [ ] Test all property CRUD operations with multiple contacts

### Optional Enhancements

- [ ] Add drag-and-drop reordering for contacts
- [ ] Add "primary contact" designation
- [ ] Add contact role/type field (Owner, Agent, Manager, etc.)
- [ ] Add inline contact editing in property form
- [ ] Add contact search/filter in selection UI

---

## Testing Guide

### Test Scenarios

1. **Create property with single existing contact**
   - Select one contact from list
   - Submit form
   - Verify property created with contactIds array of length 1

2. **Create property with multiple existing contacts**
   - Select 2-3 contacts from list
   - Submit form
   - Verify property created with multiple contactIds

3. **Create property with single new contact**
   - Fill in new contact form
   - Submit
   - Verify new contact created and linked

4. **Create property with multiple new contacts**
   - Add 2-3 new contact forms
   - Fill in all fields
   - Submit
   - Verify all contacts created and linked

5. **Create property with mix of existing and new contacts**
   - Select 1 existing contact
   - Add 1 new contact
   - Submit
   - Verify both contacts linked

6. **Edit property - change contacts**
   - Load existing property
   - Remove one contact
   - Add another contact
   - Save
   - Verify contacts updated

7. **Validation - no contacts selected**
   - Try to submit without any contacts
   - Verify validation error shown

8. **Display - property list**
   - Load property list
   - Verify all contacts shown for each property

9. **Display - property detail**
   - Open property detail page
   - Verify all contacts displayed with full information

---

## Summary

The N:N relationship allows much more flexibility in property management:

- **Multiple owners** per property
- **Multiple agents** per property
- **Mix of owner + agent contacts**
- Each **contact can be reused** across multiple properties

This improves data organization and reduces duplication while providing users with more comprehensive contact information for each property.
