# Ration Rotation
A Go-based inventory management system for tracking perishable goods with expiration date monitoring and Signal notification integration.

![Go](https://img.shields.io/badge/Go-1.27+-blue) ![License](https://img.shields.io/badge/License-MIT-green)

## 🌟 Features

- **Expiration Tracking** - Visual indicators for items approaching expiration
- **Image Support** - Attach product images to inventory items (file upload or base64)
- **Barcode Support** - Track and lookup items by barcode/UPC
- **Signal Integration** - Send Signal notifications for expiring items
- **Health Checks** - `/health` endpoint for load balancer monitoring
- **Automated Warnings** - Cron-based checks for expiring items (configurable interval)
- **Manual Warning Trigger** - On-demand warning check endpoint

## 📁 Project Structure

```
ration-rotation/
├── backend/                    # Go API server
│   ├── cmd/server/            # Application entry point
│   ├── internal/
│   │   ├── api/               # API handlers and services
│   │   ├── config/            # Configuration loading
│   │   ├── db/                # Database connections and schema
│   │   ├── models/            # Data models (Item)
│   │   ├── store/             # Repository layer
│   │   └── signal/            # Signal notification integration
│   ├── config.json            # Server configuration
│   ├── Dockerfile
│   └── go.mod
├── signal-config/             # Signal CLI configuration
└── docker-compose.yml
```

### Frontend
Coming soon!

## 🚀 Quick Start

### Option 1: Run Backend Only (Development)

```bash
cd backend
go run cmd/server/main.go
```

Server will be available at `http://localhost:8080`

### Option 2: Docker Compose (All Services)

```bash
docker-compose up -d
# Backend: http://localhost:8080
# PostgreSQL: localhost:5432
```

## 📚 API Endpoints

### Health & System

| Method | Endpoint | Description | Request Body | Response |
|--------|----------|-------------|---------------|----------|
| `GET` | `/health` | Health check for load balancers | None | `{"status": "ok"}` |

### Items Management (`/api/items`)

| Method | Endpoint | Description | Query Params | Request Body (JSON) | Response |
|--------|----------|-------------|---------------|---------------------|----------|
| `GET` | `/items` | List all items | `?offset=0&limit=100` | None | Array of items |
| `POST` | `/items` | Create new item | None | See below | Created item |
| `GET` | `/items/barcode/:barcode` | Get item by barcode | None | None | Item or 404 |
| `GET` | `/items/:id` | Get item by ID | None | None | Item or 404 |
| `PATCH` | `/items/:id` | Update item | None | Partial item data | Updated item |
| `DELETE` | `/items/:id` | Delete item | None | None | 204 No Content |
| `POST` | `/items/:id/image` | Upload image | `multipart/form-data: image` or `?image=<base64>` | File or base64 string | Success message with item |
| `PATCH` | `/items/:id/snooze` | Snooze expiration warning | `?days=7` | None | Updated item with new expiry |
| `PATCH` | `/items/:id/rotate` | Rotate item dates | None | JSON: `{new_best_before_date, stock_date, quantity}` | Updated item |

### Warnings (`/api/warnings`)

| Method | Endpoint | Description | Request Body | Response |
|--------|----------|-------------|---------------|----------|
| `GET` | `/warnings` | Get items expiring soon | None | Array of expiring items |
| `GET` | `/warnings/check` | Trigger immediate warning check | None | Success message |

### Signal Integration (`/api/link`)

| Method | Endpoint | Description | Request Body | Response |
|--------|----------|-------------|---------------|----------|
| `POST` | `/link` | Start linking Signal account | `{account}` | Link URL or error |
| `GET` | `/link/status` | Get current linking status | None | Status and linked state |

## 📦 Item Model

When creating or updating items, use this JSON structure:

```json
{
  "name": "Milk",                    // Required
  "quantity": 1.0,                  // Required (must be > 0)
  "description": "Whole milk",      // Optional
  "ingredients": "Pasteurized dairy", // Optional
  "stock_date": "2024-10-19T10:30:00Z", // Required (defaults to now on create)
  "best_before_date": "2024-10-26T00:00:00Z",  // Required
  "days_before_warning": 7,         // Optional (default: 7)
  "image": null,                    // Optional (file upload or base64)
  "barcode": "123456789"            // Optional
}
```

### Item Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `id` | UUID | Auto-generated | Primary key (UUID from database) |
| `name` | string | ✅ Yes | Item name (mandatory for creation) |
| `quantity` | float | ✅ Yes | Quantity of the item (must be > 0) |
| `description` | string | ❌ No | Additional description |
| `ingredients` | string | ❌ No | Ingredients list |
| `stock_date` | datetime | On create only | When this stock was received |
| `best_before_date` | date | ✅ Yes | Expiration date (required on creation) |
| `days_before_warning` | int | ❌ No | Days before expiry to trigger warning (default: 7) |
| `image` | bytes | ❌ No | Product image (stored in DB) |
| `barcode` | string | ❌ No | Barcode/UPC for quick lookup |
| `created_at` | datetime | Auto-generated | Record creation timestamp |
| `updated_at` | datetime | Auto-generated | Record update timestamp |

## 🛠️ Tech Stack (Backend)

- **Go 1.21+** - Language and runtime
- **Gin** - Web framework for REST API
- **PostgreSQL** - Relational database via lib/pq driver
- **Signal-CLI** - SMS notification integration
- **go-cron** - Scheduled job scheduling

## 🔐 Security Features

- **Rate Limiting** - 10 requests/second with burst of 20 per IP
- **CORS Protection** - Configurable allowed origins
- **Graceful Shutdown** - Proper cleanup on SIGTERM/SIGINT

## 🌐 Environment Variables

Create a `.env` file in the backend directory:

```bash
DATABASE_URL=postgresql://user:password@localhost:5432/ration_rotation?sslmode=disable
PORT=8080
TZ=Europe/Stockholm
SIGNAL_ENDPOINT=http://signal-cli:7583  # Optional, if using Signal notifications
SIGNAL_TO=+1234567890                   # Optional phone number for notifications
```

Or use the `config.json` file (see `config-example.json` template).

## ⚙️ Configuration

The backend uses a JSON config file with these sections:

### Database Section
- `url`: PostgreSQL connection string
- `create_tables_if_missing`: Auto-create items table (default: true)

### Server Section
- `address`: Bind address (default: 0.0.0.0)
- `port`: Server port (default: 8080)
- `cors_allowed_origins`: List of allowed CORS origins
- `tz`: Timezone for date formatting (default: Europe/Stockholm)

### Warning Section
- `check_interval_minutes`: How often to check for expiring items (default: 45 minutes)

### Signal Section (Optional)
- `endpoint`: Signal-cli JSON-RPC endpoint URL
- `to`: Phone number(s) to receive notifications

## 📝 Best Practices

- Always verify backend health with `/health` before using API endpoints
- Use pagination (`offset`/`limit`) when listing items for large datasets
- Include meaningful error handling in your frontend for network failures
- Validate required fields (name, quantity, best_before_date) client-side before sending requests
- Handle both file uploads and base64 images for the `/items/:id/image` endpoint

## 🤝 Contributing

To extend this backend:

1. Add new handlers in `internal/api/`
2. Create new models in `internal/models/`
3. Update store methods in `internal/store/`
4. Test database changes with a running container

## 📄 License

MIT License - feel free to use and modify as needed.
