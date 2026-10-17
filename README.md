# Vigie

A simple, self-hosted status page for small teams or personal projects. Built as a learning project to explore Go, React, and Docker.

**Note:** This is a personal project, not production-grade software. Use at your own risk.

## What it does

- Public status page showing component health and incidents
- Admin dashboard to manage components, incidents, and maintenance windows
- Email notifications to subscribers (SMTP, SendGrid, Mailgun, or AWS SES)
- Bilingual support (English/French)

## Tech Stack

- **API**: Go + Gin + GORM
- **Frontend/Admin**: React + TypeScript + Vite + Tailwind CSS
- **Database**: MySQL 8.4
- **Deployment**: Docker Compose or Kubernetes (Helm)

## Quick Start

```bash
# Clone and start
git clone <repository-url>
cd vigie
cp .env.example .env
# Edit .env to set JWT_SECRET_KEY (use: openssl rand -base64 32)
docker-compose up -d
```

Access:
- Status page: http://localhost:3000
- Admin: http://localhost:3001 (default: admin / admin)
- API: http://localhost:8080/api/v1

**Important:** Change the default admin password after first login.

## Configuration

Key environment variables (see `.env.example` for all options):

| Variable | Description |
|----------|-------------|
| `JWT_SECRET_KEY` | Secret for JWT tokens (required, change in production) |
| `DB_*` | Database connection settings |
| `EMAIL_PROVIDER` | `disabled`, `smtp`, `sendgrid`, `mailgun`, or `ses` |

## Project Structure

```
vigie/
├── api/           # Go backend
├── frontend/      # Public status page (React)
├── admin/         # Admin interface (React)
├── helm/          # Helm chart for Kubernetes
├── docs/          # Documentation
└── docker-compose.yml
```

## Documentation

- [API Reference](docs/API.md) - Full API documentation
- [Kubernetes Deployment](docs/KUBERNETES.md) - Helm chart deployment guide
- [Cachet compatible API](docs/CACHET_API.md) - For tools written for Cachet 2.x
- [Migrating from Cachet](docs/MIGRATING_FROM_CACHET.md) - Step-by-step guide to move a Cachet 2.4 status page
- [Importing from Cachet](docs/IMPORT_CACHET.md) - Move a Cachet 2.4 installation to Vigie

## Docker Commands

```bash
docker-compose up -d          # Start
docker-compose down           # Stop
docker-compose logs -f api    # View logs
docker-compose up -d --build  # Rebuild after changes
docker-compose down -v        # Reset (deletes data!)
```

## Known Limitations

- No real-time updates (polling only)
- No external monitoring integration
- Basic email templates
- Limited to English and French
- No automated backups

If you're interested in more features, feel free to open an issue or reach out!

## License

MIT License. See [LICENSE](LICENSE) for details.
