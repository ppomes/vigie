# Vigie API Documentation

Base URL: `http://localhost:8080/api/v1`

## Table of Contents

- [Authentication](#authentication)
- [Rate Limiting](#rate-limiting)
- [Public Endpoints](#public-endpoints)
  - [Status](#status)
  - [Components](#components)
  - [Incidents](#incidents)
  - [Maintenances](#maintenances)
  - [Subscriptions](#subscriptions)
  - [Settings](#settings)
- [Admin Endpoints](#admin-endpoints)
  - [Authentication](#authentication-1)
  - [User Profile](#user-profile)
  - [Two-Factor Login](#two-factor-login)
  - [Components Management](#components-management)
  - [Incidents Management](#incidents-management)
  - [Subscribers Management](#subscribers-management)
  - [Maintenances Management](#maintenances-management)
  - [Templates](#templates)
  - [Maintenance Templates](#maintenance-templates)
  - [Settings Management](#settings-management)
  - [Email Configuration](#email-configuration)
  - [Notifications](#notifications)
  - [User Management](#user-management)
  - [Audit Logs](#audit-logs)

---

## Authentication

Admin endpoints require a JWT token in the `Authorization` header:

```
Authorization: Bearer <token>
```

Tokens are obtained via the login endpoint and expire after 24 hours.

### Roles

- **admin**: Full access to all endpoints
- **viewer**: Read-only access to admin endpoints (can view but not modify)

---

## Rate Limiting

The following endpoints are rate-limited per IP address:

| Endpoint | Limit | Window |
|----------|-------|--------|
| `POST /admin/login` | 10 requests | 15 minutes |
| `POST /forgot-password` | 3 requests | 1 hour |
| `POST /reset-password` | 3 requests | 1 hour |
| `POST /subscribe` | 10 requests | 1 hour |
| `POST /admin/email/test` | 5 requests | 1 hour |

When rate limited, the API returns:

```json
{
  "error": "Too many requests. Please try again later."
}
```

---

## Public Endpoints

### Status

#### Get System Status

Returns the overall system status based on components and active incidents.

```
GET /status
```

**Response:**

```json
{
  "status": "operational",
  "message": "All systems operational",
  "components_count": 5,
  "active_incidents_count": 0
}
```

Status values: `operational`, `degraded`, `outage`

---

### Components

#### Get All Components

Returns all components with translations for the specified locale.

```
GET /components
GET /components?locale=fr
```

**Query Parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| `locale` | string | Language code (`en` or `fr`). Also accepts `Accept-Language` header. Default: `en` |

**Response:**

```json
{
  "components": [
    {
      "id": 1,
      "name": "API Server",
      "status": "operational",
      "order": 1,
      "translations": [
        {
          "locale": "en",
          "field": "name",
          "value": "API Server"
        }
      ],
      "created_at": "2024-01-15T10:00:00Z",
      "updated_at": "2024-01-15T10:00:00Z"
    }
  ],
  "locale": "en"
}
```

Component status values: `operational`, `degraded_performance`, `partial_outage`, `major_outage`

---

### Incidents

#### Get All Incidents

Returns all incidents with optional status filtering.

```
GET /incidents
GET /incidents?status=investigating
GET /incidents?locale=fr
```

**Query Parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| `status` | string | Filter by status: `investigating`, `identified`, `monitoring`, `resolved` |
| `locale` | string | Language code (`en` or `fr`). Default: `en` |

**Response:**

```json
{
  "incidents": [
    {
      "id": 1,
      "name": "Database connectivity issues",
      "status": "investigating",
      "component_id": 2,
      "component": {
        "id": 2,
        "name": "Database",
        "status": "major_outage"
      },
      "translations": [],
      "created_at": "2024-01-15T14:30:00Z",
      "updated_at": "2024-01-15T14:35:00Z"
    }
  ],
  "locale": "en"
}
```

Incident status values: `investigating`, `identified`, `monitoring`, `resolved`

---

### Maintenances

#### Get All Maintenances

Returns all scheduled and active maintenances.

```
GET /maintenances
GET /maintenances?locale=fr
```

**Query Parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| `locale` | string | Language code (`en` or `fr`). Default: `en` |

**Response:**

```json
{
  "maintenances": [
    {
      "id": 1,
      "name": "Scheduled database maintenance",
      "status": "scheduled",
      "scheduled_start": "2024-01-20T02:00:00Z",
      "scheduled_end": "2024-01-20T04:00:00Z",
      "component_id": 2,
      "translations": [],
      "created_at": "2024-01-15T10:00:00Z"
    }
  ],
  "locale": "en"
}
```

Maintenance status values: `scheduled`, `in_progress`, `completed`

---

### Subscriptions

#### Subscribe to Updates

Subscribe an email address to receive status notifications.

```
POST /subscribe
```

**Request:**

```json
{
  "email": "user@example.com",
  "locale": "en"
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `email` | string | Yes | Valid email address |
| `locale` | string | No | Preferred language (`en` or `fr`). Default: `en` |

**Response (201):**

```json
{
  "message": "Please check your email to confirm your subscription",
  "email": "user@example.com"
}
```

#### Confirm Subscription

Confirm email subscription using the token sent via email.

```
GET /confirm?token=<confirmation_token>
```

**Response (200):**

```json
{
  "message": "Email confirmed successfully"
}
```

#### Unsubscribe

Unsubscribe from status notifications.

```
GET /unsubscribe?token=<unsubscribe_token>
```

**Response (200):**

```json
{
  "message": "You have been unsubscribed successfully"
}
```

---

### Settings

#### Get Theme

Returns the current theme setting.

```
GET /settings/theme
```

**Response:**

```json
{
  "theme": "default"
}
```

Theme values: `default`, `dark`, `corporate`, `modern`

#### Get Branding

Returns branding settings (custom title and logo).

```
GET /settings/branding
```

**Response:**

```json
{
  "title_en": "My Status Page",
  "title_fr": "Ma Page de Statut",
  "logo": "data:image/png;base64,..."
}
```

---

## Admin Endpoints

All admin endpoints require authentication via JWT token.

### Authentication

#### Login

Authenticate and obtain a JWT token.

```
POST /admin/login
```

**Request:**

```json
{
  "username": "admin",
  "password": "your_password"
}
```

**Response (200):**

```json
{
  "token": "eyJhbGciOiJIUzI1NiIs...",
  "expires_at": "2024-01-16T14:30:00Z",
  "user": {
    "id": 1,
    "username": "admin",
    "email": "admin@example.com",
    "role": "admin"
  },
  "force_password_change": false
}
```

A user with two-factor login on must also send `code`: the six-digit code
from their authenticator app, or one of their recovery codes. Without it, or
with a wrong one, the answer is `401` with `"two_factor_required": true`, and
the client asks for the code and sends the login again with it. Each code
works once. A login with a recovery code also returns `recovery_codes_left`.

```json
{
  "username": "admin",
  "password": "your_password",
  "code": "123456"
}
```

**Error Responses:**

- `401 Unauthorized`: Invalid credentials, or two-factor code missing or wrong
  (`two_factor_required` is then `true`)
- `403 Forbidden`: Account deactivated
- `429 Too Many Requests`: Rate limited

#### Forgot Password

Request a password reset email.

```
POST /forgot-password
```

**Request:**

```json
{
  "email": "admin@example.com"
}
```

**Response (200):**

```json
{
  "message": "If an account with that email exists, a password reset link has been sent"
}
```

Note: Always returns success for security (doesn't reveal if email exists).

#### Reset Password

Reset password using the token from the email.

```
POST /reset-password
```

**Request:**

```json
{
  "token": "abc123...",
  "password": "NewSecurePassword123!"
}
```

**Password Requirements:**
- Minimum 12 characters
- At least one uppercase letter
- At least one lowercase letter
- At least one digit
- At least one special character
- Cannot contain username

**Response (200):**

```json
{
  "message": "Password has been reset successfully"
}
```

---

### User Profile

#### Get Current User

Returns the authenticated user's profile.

```
GET /admin/me
```

**Response:**

```json
{
  "user": {
    "id": 1,
    "username": "admin",
    "email": "admin@example.com",
    "role": "admin",
    "active": true,
    "last_login": "2024-01-15T10:00:00Z",
    "created_at": "2024-01-01T00:00:00Z",
    "two_factor_enabled": true,
    "recovery_codes_left": 8
  }
}
```

`recovery_codes_left` is only present when two-factor login is on.

#### Change Own Password

Change the authenticated user's password.

```
PUT /admin/me/password
```

**Request:**

```json
{
  "old_password": "current_password",
  "new_password": "NewSecurePassword123!"
}
```

**Response (200):**

```json
{
  "message": "Password changed successfully"
}
```

### Two-Factor Login

Any user can turn on two-factor login (TOTP, the codes of Google
Authenticator, 1Password, Aegis and similar apps). These endpoints need an
interactive session: an API key cannot use them. API keys keep working
without a code.

#### Start the Setup

```
POST /admin/me/two-factor/setup
```

Request: `{"password": "current_password"}`. Returns a new secret, its
`otpauth://` address, and a QR code of that address as a PNG data URL. Login
does not change until the setup is confirmed. `409` if two-factor login is
already on.

```json
{
  "secret": "JBSWY3DPEHPK3PXPJBSWY3DPEHPK3PXP",
  "otpauth_url": "otpauth://totp/Vigie:admin?algorithm=SHA1&digits=6&issuer=Vigie&period=30&secret=...",
  "qr_code": "data:image/png;base64,..."
}
```

The app shows the status page's English title, or "Vigie", next to the code.

#### Confirm the Setup

```
POST /admin/me/two-factor/enable
```

Request: `{"code": "123456"}`, a code from the app. Turns two-factor login on
and returns eight recovery codes. They are shown only once; each one replaces a
code once, for a user who lost their phone.

```json
{
  "recovery_codes": ["7k2m9-x4q8d", "..."]
}
```

#### New Recovery Codes

```
POST /admin/me/two-factor/recovery-codes
```

Request: `{"password": "current_password"}`. Replaces the recovery codes and
returns the new ones.

#### Turn Off

```
POST /admin/me/two-factor/disable
```

Request: `{"password": "current_password"}`. Turns two-factor login off and
deletes the recovery codes.

---

### Components Management

#### List Components (Admin)

Returns all components with all translations.

```
GET /admin/components
```

**Response:**

```json
{
  "components": [
    {
      "id": 1,
      "name": "API Server",
      "status": "operational",
      "order": 1,
      "translations": [
        {"locale": "en", "field": "name", "value": "API Server"},
        {"locale": "fr", "field": "name", "value": "Serveur API"}
      ]
    }
  ]
}
```

#### Create Component

**Admin only**

```
POST /admin/components
```

**Request:**

```json
{
  "name": "API Server",
  "status": "operational",
  "order": 1,
  "translations": [
    {"locale": "en", "field": "name", "value": "API Server"},
    {"locale": "fr", "field": "name", "value": "Serveur API"}
  ]
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | Component name |
| `status` | string | Yes | Status: `operational`, `degraded_performance`, `partial_outage`, `major_outage` |
| `order` | integer | No | Display order (lower = first) |
| `translations` | array | No | Localized content |

**Response (201):**

```json
{
  "message": "Component created successfully",
  "component": { ... }
}
```

#### Update Component

**Admin only**

```
PUT /admin/components/:id
```

**Request:**

```json
{
  "name": "API Server (Updated)",
  "status": "degraded_performance",
  "order": 2,
  "translations": [...]
}
```

All fields are optional. Only provided fields are updated.

**Response (200):**

```json
{
  "message": "Component updated successfully",
  "component": { ... }
}
```

#### Delete Component

**Admin only** (soft delete)

```
DELETE /admin/components/:id
```

**Response (200):**

```json
{
  "message": "Component deleted successfully"
}
```

---

### Incidents Management

#### List Incidents (Admin)

Returns all incidents with translations and components.

```
GET /admin/incidents
```

**Response:**

```json
{
  "incidents": [
    {
      "id": 1,
      "name": "Database issues",
      "status": "investigating",
      "component_id": 2,
      "component": { ... },
      "translations": [ ... ]
    }
  ]
}
```

#### Create Incident

**Admin only**

```
POST /admin/incidents
```

**Request:**

```json
{
  "name": "Database connectivity issues",
  "status": "investigating",
  "component_id": 2,
  "translations": [
    {"locale": "en", "field": "name", "value": "Database connectivity issues"},
    {"locale": "en", "field": "message", "value": "We are investigating database connection problems."},
    {"locale": "fr", "field": "name", "value": "Problèmes de connectivité base de données"},
    {"locale": "fr", "field": "message", "value": "Nous enquêtons sur des problèmes de connexion à la base de données."}
  ]
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | Incident name |
| `status` | string | Yes | Status: `investigating`, `identified`, `monitoring`, `resolved` |
| `component_id` | integer | Yes | Affected component ID |
| `translations` | array | No | Localized content (fields: `name`, `message`) |

Note: Creating an incident automatically updates the component status.

**Response (201):**

```json
{
  "message": "Incident created successfully",
  "incident": { ... }
}
```

#### Update Incident

**Admin only**

```
PUT /admin/incidents/:id
```

**Request:**

```json
{
  "name": "Database issues - Update",
  "status": "identified",
  "component_id": 2,
  "translations": [...]
}
```

**Response (200):**

```json
{
  "message": "Incident updated successfully",
  "incident": { ... }
}
```

#### Delete Incident

**Admin only** (soft delete)

```
DELETE /admin/incidents/:id
```

**Response (200):**

```json
{
  "message": "Incident deleted successfully"
}
```

---

### Subscribers Management

#### List Subscribers

```
GET /admin/subscribers
```

**Response:**

```json
{
  "subscribers": [
    {
      "id": 1,
      "email": "user@example.com",
      "confirmed": true,
      "locale": "en",
      "created_at": "2024-01-15T10:00:00Z"
    }
  ]
}
```

#### Delete Subscriber

**Admin only** (soft delete)

```
DELETE /admin/subscribers/:id
```

**Response (200):**

```json
{
  "message": "Subscriber deleted successfully"
}
```

---

### Maintenances Management

#### List Maintenances (Admin)

```
GET /admin/maintenances
```

**Response:**

```json
{
  "maintenances": [...]
}
```

#### Create Maintenance

**Admin only**

```
POST /admin/maintenances
```

**Request:**

```json
{
  "name": "Scheduled database maintenance",
  "status": "scheduled",
  "scheduled_start": "2024-01-20T02:00:00Z",
  "scheduled_end": "2024-01-20T04:00:00Z",
  "component_id": 2,
  "translations": [
    {"locale": "en", "field": "name", "value": "Scheduled database maintenance"},
    {"locale": "en", "field": "message", "value": "We will be performing scheduled maintenance."}
  ]
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | Maintenance name |
| `status` | string | Yes | Status: `scheduled`, `in_progress`, `completed` |
| `scheduled_start` | datetime | Yes | Start time (ISO 8601) |
| `scheduled_end` | datetime | Yes | End time (ISO 8601) |
| `component_id` | integer | Yes | Affected component ID |
| `translations` | array | No | Localized content |

**Response (201):**

```json
{
  "message": "Maintenance created successfully",
  "maintenance": { ... }
}
```

#### Update Maintenance

**Admin only**

```
PUT /admin/maintenances/:id
```

#### Delete Maintenance

**Admin only**

```
DELETE /admin/maintenances/:id
```

---

### Templates

Incident templates for quick incident creation.

#### List Templates

```
GET /admin/templates
```

#### Get Template

```
GET /admin/templates/:id
```

#### Create Template

**Admin only**

```
POST /admin/templates
```

**Request:**

```json
{
  "name": "Database Outage Template",
  "incident_name": "Database Outage",
  "incident_status": "investigating",
  "translations": [...]
}
```

#### Update Template

**Admin only**

```
PUT /admin/templates/:id
```

#### Delete Template

**Admin only**

```
DELETE /admin/templates/:id
```

#### Preview Template

**Admin only**

```
POST /admin/templates/:id/preview
```

---

### Maintenance Templates

#### List Maintenance Templates

```
GET /admin/maintenance-templates
```

#### Get Maintenance Template

```
GET /admin/maintenance-templates/:id
```

#### Create Maintenance Template

**Admin only**

```
POST /admin/maintenance-templates
```

#### Update Maintenance Template

**Admin only**

```
PUT /admin/maintenance-templates/:id
```

#### Delete Maintenance Template

**Admin only**

```
DELETE /admin/maintenance-templates/:id
```

#### Preview Maintenance Template

**Admin only**

```
POST /admin/maintenance-templates/:id/preview
```

---

### Settings Management

#### List Settings

```
GET /admin/settings
```

**Response:**

```json
{
  "settings": [
    {"id": 1, "key": "theme", "value": "default"},
    {"id": 2, "key": "email_config", "value": "{...}"}
  ]
}
```

#### Update Setting

**Admin only**

```
PUT /admin/settings/:key
```

**Request:**

```json
{
  "value": "dark"
}
```

**Response (200):**

```json
{
  "id": 1,
  "key": "theme",
  "value": "dark"
}
```

#### Update Branding

**Admin only**

```
PUT /admin/settings/branding
```

**Request:**

```json
{
  "title_en": "My Status Page",
  "title_fr": "Ma Page de Statut",
  "logo": "data:image/png;base64,..."
}
```

| Field | Type | Description |
|-------|------|-------------|
| `title_en` | string | English page title (empty = use default) |
| `title_fr` | string | French page title (empty = use default) |
| `logo` | string | Base64 data URL (max 500KB, PNG/JPG/SVG/GIF/WebP) |

**Response (200):**

```json
{
  "message": "Branding settings saved",
  "config": { ... }
}
```

---

### Email Configuration

#### Get Email Config

```
GET /admin/email
```

**Response:**

```json
{
  "config": {
    "provider": "smtp",
    "from_address": "noreply@example.com",
    "from_name": "Status Page",
    "smtp_host": "smtp.example.com",
    "smtp_port": 587,
    "smtp_username": "user",
    "smtp_password": "********",
    "smtp_tls": false
  }
}
```

Note: Sensitive fields (passwords, API keys) are masked.

#### Update Email Config

**Admin only**

```
PUT /admin/email
```

**Request (SMTP):**

```json
{
  "provider": "smtp",
  "from_address": "noreply@example.com",
  "from_name": "Status Page",
  "smtp_host": "smtp.example.com",
  "smtp_port": 587,
  "smtp_username": "user",
  "smtp_password": "password",
  "smtp_tls": false
}
```

**Request (SendGrid):**

```json
{
  "provider": "sendgrid",
  "from_address": "noreply@example.com",
  "from_name": "Status Page",
  "sendgrid_api_key": "SG.xxxxx"
}
```

**Request (Mailgun):**

```json
{
  "provider": "mailgun",
  "from_address": "noreply@example.com",
  "from_name": "Status Page",
  "mailgun_api_key": "key-xxxxx",
  "mailgun_domain": "mg.example.com"
}
```

**Request (AWS SES):**

```json
{
  "provider": "ses",
  "from_address": "noreply@example.com",
  "from_name": "Status Page",
  "aws_region": "us-east-1",
  "aws_access_key": "AKIAXXXXX",
  "aws_secret_key": "secret"
}
```

Provider values: `disabled`, `smtp`, `sendgrid`, `mailgun`, `ses`

#### Test Email Config

**Admin only**

```
POST /admin/email/test
```

**Request:**

```json
{
  "email": "test@example.com"
}
```

**Response (200):**

```json
{
  "message": "Test email sent successfully"
}
```

---

### Notifications

#### Get Notification Stats

```
GET /admin/notifications/stats
```

**Response:**

```json
{
  "pending": 0,
  "sent": 150,
  "failed": 2,
  "total": 152
}
```

#### Get Recent Notifications

```
GET /admin/notifications/recent
GET /admin/notifications/recent?limit=50
```

**Query Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `limit` | integer | 100 | Number of notifications to return |

#### Retry Failed Notifications

**Admin only**

```
POST /admin/notifications/retry
```

**Response (200):**

```json
{
  "message": "Retrying 2 failed notifications"
}
```

---

### User Management

**Admin only** - All user management endpoints require admin role.

#### List Users

```
GET /admin/users
```

**Response:**

```json
{
  "users": [
    {
      "id": 1,
      "username": "admin",
      "email": "admin@example.com",
      "role": "admin",
      "active": true,
      "last_login": "2024-01-15T10:00:00Z",
      "created_at": "2024-01-01T00:00:00Z"
    }
  ]
}
```

#### Get User

```
GET /admin/users/:id
```

#### Create User

```
POST /admin/users
```

**Request:**

```json
{
  "username": "newuser",
  "email": "newuser@example.com",
  "password": "SecurePassword123!",
  "role": "viewer"
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `username` | string | Yes | 3-255 characters |
| `email` | string | Yes | Valid email address |
| `password` | string | Yes | Min 12 chars, must meet strength requirements |
| `role` | string | Yes | `admin` or `viewer` |

**Response (201):**

```json
{
  "user": { ... }
}
```

#### Update User

```
PUT /admin/users/:id
```

**Request:**

```json
{
  "email": "updated@example.com",
  "role": "admin",
  "active": true
}
```

All fields are optional.

Note: Admins cannot demote themselves or deactivate their own account.

#### Delete User

```
DELETE /admin/users/:id
```

Note: Users cannot delete their own account.

#### Reset User Password

```
PUT /admin/users/:id/password
```

**Request:**

```json
{
  "password": "NewSecurePassword123!"
}
```

Sets `force_password_change` flag - user must change password on next login.

#### Reset User Two-Factor Login

```
DELETE /admin/users/:id/two-factor
```

Turns two-factor login off for a user who lost their phone and their recovery
codes. They can turn it on again from their profile.

---

### Audit Logs

**Admin only**

#### List Audit Logs

```
GET /admin/audit
GET /admin/audit?page=1&limit=50
GET /admin/audit?action=login&user_id=1
```

**Query Parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| `page` | integer | Page number (default: 1) |
| `limit` | integer | Items per page (default: 50, max: 100) |
| `action` | string | Filter by action type |
| `user_id` | integer | Filter by user ID |
| `resource` | string | Filter by resource type |
| `start_date` | datetime | Filter from date |
| `end_date` | datetime | Filter to date |

**Response:**

```json
{
  "logs": [
    {
      "id": 1,
      "user_id": 1,
      "username": "admin",
      "action": "login",
      "resource": "auth",
      "resource_id": null,
      "details": {"ip": "192.168.1.1"},
      "success": true,
      "ip_address": "192.168.1.1",
      "created_at": "2024-01-15T10:00:00Z"
    }
  ],
  "total": 100,
  "page": 1,
  "limit": 50
}
```

#### Export Audit Logs

```
GET /admin/audit/export
GET /admin/audit/export?format=csv
```

**Query Parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| `format` | string | Export format: `json` (default) or `csv` |
| (same filters as List Audit Logs) |

Returns downloadable file with `Content-Disposition` header.

---

## Error Responses

All error responses follow this format:

```json
{
  "error": "Error message here",
  "details": "Optional additional details"
}
```

### Common HTTP Status Codes

| Code | Description |
|------|-------------|
| `200` | Success |
| `201` | Created |
| `400` | Bad Request - Invalid input |
| `401` | Unauthorized - Missing or invalid token |
| `403` | Forbidden - Insufficient permissions |
| `404` | Not Found - Resource doesn't exist |
| `409` | Conflict - Resource already exists |
| `429` | Too Many Requests - Rate limited |
| `500` | Internal Server Error |

---

## Translations

Components, incidents, and maintenances support translations for multilingual content.

### Translation Object

```json
{
  "locale": "en",
  "field": "name",
  "value": "API Server"
}
```

| Field | Type | Description |
|-------|------|-------------|
| `locale` | string | Language code: `en` or `fr` |
| `field` | string | Field name: `name`, `message`, `description` |
| `value` | string | Translated content |

### Supported Locales

- `en` - English (default)
- `fr` - French
