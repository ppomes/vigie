# OpenStatus

A modern, self-hosted status page system with real-time monitoring and incident management capabilities. OpenStatus provides a clean, customizable status page for your services along with a powerful admin interface to manage incidents, maintenance windows, and system components.

## 🚀 Features

- **Public Status Page**: Beautiful, responsive status page for your users
- **Admin Dashboard**: Comprehensive admin interface for managing your status page
- **Real-time Updates**: Keep your users informed with live status updates
- **Incident Management**: Create and manage incidents with timeline tracking
- **Email Notifications**: Multi-provider email support (SendGrid, Mailgun, AWS SES)
- **JWT Authentication**: Secure API access with JWT tokens
- **Multi-language Support**: Built-in internationalization (i18n)
- **Docker Compose Setup**: Easy deployment with Docker
- **Health Checks**: Automated service health monitoring

## 🏗️ Architecture

OpenStatus is built with a modern microservices architecture:

- **API Service** (Go): RESTful API backend with Gin framework
- **Frontend** (Next.js): Public-facing status page
- **Admin** (Next.js): Administrative interface
- **Database** (MySQL 8.4): Persistent data storage

All services are containerized and orchestrated using Docker Compose for easy deployment and scalability.

## 🛠️ Technology Stack

### Backend (API)
- **Go 1.23**: High-performance backend language
- **Gin Framework**: Fast HTTP web framework
- **GORM**: ORM library for database interactions
- **JWT**: JSON Web Tokens for authentication
- **Cron**: Scheduled job execution

### Frontend & Admin
- **Next.js**: React framework with SSR/SSG
- **TypeScript**: Type-safe JavaScript
- **Tailwind CSS**: Utility-first CSS framework
- **next-intl**: Internationalization support
- **Vitest**: Unit testing framework

### Database
- **MySQL 8.4**: Relational database with utf8mb4 support

### DevOps
- **Docker**: Containerization
- **Docker Compose**: Multi-container orchestration

## 📋 Prerequisites

Before you begin, ensure you have the following installed:

- **Docker**: Version 20.10 or higher
- **Docker Compose**: Version 2.0 or higher

For local development without Docker:
- **Go**: Version 1.23 or higher
- **Node.js**: Version 18 or higher
- **MySQL**: Version 8.4 or compatible

## 🚀 Getting Started

### Quick Start with Docker Compose

1. **Clone the repository**
   ```bash
   git clone <repository-url>
   cd openstatus
   ```

2. **Configure environment variables**
   ```bash
   cp .env.example .env
   ```

   Edit `.env` and update the following critical values:
   - `JWT_SECRET_KEY`: Generate a secure key with `openssl rand -base64 32`
   - `EMAIL_PROVIDER`: Choose your email provider (sendgrid, mailgun, or ses)
   - `SENDGRID_API_KEY` / `MAILGUN_API_KEY` / AWS credentials: Add your email provider credentials
   - `EMAIL_FROM`: Set your sender email address

3. **Start all services**
   ```bash
   docker-compose up -d
   ```

4. **Verify services are running**
   ```bash
   docker-compose ps
   ```

5. **Access the application**
   - **Frontend (Public Status Page)**: http://localhost:3000
   - **Admin Dashboard**: http://localhost:3001
   - **API**: http://localhost:8080/api/v1

### Initial Setup

The API service will automatically run database migrations on first startup. The database schema includes tables for:
- User authentication
- Service components
- Incidents and updates
- Maintenance windows
- Email notifications

## 🔧 Development Setup

### Running Individual Services

#### API Service
```bash
cd api
cp .env.example .env
# Edit .env with your local MySQL settings
go mod download
go run cmd/server/main.go
```

#### Frontend Service
```bash
cd frontend
cp .env.local.example .env.local
npm install
npm run dev
```

#### Admin Service
```bash
cd admin
cp .env.local.example .env.local
npm install
npm run dev
```

### Running Tests

#### API Tests
```bash
cd api
go test ./...
```

#### Frontend/Admin Tests
```bash
cd frontend  # or cd admin
npm test
```

## 📁 Project Structure

```
openstatus/
├── api/                    # Go API service
│   ├── cmd/               # Application entrypoints
│   ├── internal/          # Internal application code
│   ├── migrations/        # Database migration files
│   ├── Dockerfile         # API service Docker image
│   └── go.mod             # Go dependencies
├── frontend/              # Public status page (Next.js)
│   ├── app/              # Next.js app directory
│   ├── components/       # React components
│   ├── lib/              # Utility libraries
│   ├── messages/         # i18n translations
│   └── Dockerfile        # Frontend Docker image
├── admin/                 # Admin interface (Next.js)
│   ├── app/              # Next.js app directory
│   ├── components/       # React components
│   ├── lib/              # Utility libraries
│   ├── messages/         # i18n translations
│   └── Dockerfile        # Admin Docker image
├── docker-compose.yml     # Docker Compose orchestration
├── .env.example           # Environment variables template
└── README.md             # This file
```

## ⚙️ Environment Configuration

The application is configured using environment variables. See `.env.example` for all available options:

### Key Configuration Options

| Variable | Description | Default |
|----------|-------------|---------|
| `DB_HOST` | MySQL database host | `mysql` |
| `DB_PORT` | MySQL database port | `3306` |
| `DB_NAME` | Database name | `statuspage` |
| `API_PORT` | API service port | `8080` |
| `FRONTEND_PORT` | Frontend service port | `3000` |
| `ADMIN_PORT` | Admin service port | `3001` |
| `JWT_SECRET_KEY` | JWT signing secret | **Must change in production** |
| `EMAIL_PROVIDER` | Email provider (sendgrid/mailgun/ses) | `sendgrid` |
| `GIN_MODE` | Gin framework mode (debug/release) | `debug` |

### Security Considerations

1. **Never commit** the `.env` file to version control
2. Use **strong, unique values** for `JWT_SECRET_KEY` in production
3. **Rotate secrets** periodically
4. Use **environment-specific** API keys (separate for dev/staging/prod)
5. Enable **SSL/TLS** in production (set `GIN_MODE=release`)
6. Restrict **database access** to application services only

## 🐳 Docker Commands

### Start all services
```bash
docker-compose up -d
```

### Stop all services
```bash
docker-compose down
```

### View logs
```bash
# All services
docker-compose logs -f

# Specific service
docker-compose logs -f api
docker-compose logs -f frontend
docker-compose logs -f admin
```

### Rebuild services after code changes
```bash
docker-compose up -d --build
```

### Reset database (WARNING: Deletes all data)
```bash
docker-compose down -v
docker-compose up -d
```

## 🔍 Health Checks

All services include health check endpoints:

- **API**: `http://localhost:8080/api/v1/status`
- **Frontend**: `http://localhost:3000`
- **Admin**: `http://localhost:3001`
- **MySQL**: Internal health check via `mysqladmin ping`

Docker Compose automatically monitors these endpoints and will restart services if they become unhealthy.

## 🌍 Internationalization

Both the frontend and admin interfaces support multiple languages:

- Translations are stored in the `messages/` directory
- Default language can be configured in `i18n.ts`
- Language selection is automatic based on browser settings

## 🤝 Contributing

Contributions are welcome! Please follow these guidelines:

1. **Fork the repository** and create your feature branch
   ```bash
   git checkout -b feature/amazing-feature
   ```

2. **Make your changes** following the existing code style
   - Go: Follow Go best practices and run `go fmt`
   - TypeScript: Follow the existing ESLint configuration

3. **Test your changes**
   ```bash
   # Run tests for modified services
   go test ./...  # for API changes
   npm test       # for frontend/admin changes
   ```

4. **Commit your changes** with descriptive messages
   ```bash
   git commit -m "Add amazing feature"
   ```

5. **Push to your fork** and submit a pull request
   ```bash
   git push origin feature/amazing-feature
   ```

### Development Guidelines

- Write clear, descriptive commit messages
- Include tests for new features
- Update documentation for API changes
- Follow the existing code organization and patterns
- Ensure all health checks pass before submitting PR

## 📝 License

This project's license information is not yet specified. Please contact the project maintainers for licensing details.

## 🆘 Support

For issues, questions, or contributions:

1. **Check existing issues** in the repository
2. **Create a new issue** with detailed information about your problem or suggestion
3. **Join discussions** to connect with other users and contributors

## 🗺️ Roadmap

Future enhancements planned:

- Webhook integrations for external monitoring tools
- Slack/Discord/Teams notifications
- Custom domain support for status pages
- Advanced analytics and uptime tracking
- API client libraries for popular languages
- Mobile applications (iOS/Android)

---

**Built with ❤️ using Go, Next.js, and modern DevOps practices**
