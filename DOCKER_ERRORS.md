# Docker Compose Build Errors

## Summary

Date: 2026-01-13
Environment: Clean Docker environment (volumes removed)
Command: `docker-compose build --no-cache`

**Build Order Attempted:**
1. MySQL (skipped - uses official image)
2. API Service
3. Frontend Service
4. Admin Service

**Result:** All three application services failed to build.

---

## Build Errors:

### 1. API Service (Go Backend)

**Status:** ❌ FAILED
**Error Type:** Missing go.sum entries for dependencies

**Error Message:**
```
internal/database/database.go:7:2: missing go.sum entry for module providing package gorm.io/driver/mysql (imported by api/internal/database); to add:
	go get api/internal/database
internal/database/database.go:8:2: missing go.sum entry for module providing package gorm.io/gorm (imported by api/internal/database); to add:
	go get api/internal/database
internal/models/user.go:6:2: missing go.sum entry for module providing package golang.org/x/crypto/bcrypt (imported by api/internal/models); to add:
	go get api/internal/models
internal/handlers/admin.go:11:2: missing go.sum entry for module providing package github.com/gin-gonic/gin; to add:
	go mod download github.com/gin-gonic/gin
internal/handlers/auth.go:11:2: missing go.sum entry for module providing package github.com/golang-jwt/jwt/v5 (imported by api/internal/handlers); to add:
	go get api/internal/handlers
cmd/api/main.go:14:2: missing go.sum entry for module providing package github.com/joho/godotenv; to add:
	go mod download github.com/joho/godotenv
```

**Build Command Failed:**
```bash
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o api ./cmd/api/main.go
```

**Exit Code:** 1

**Root Cause:** The `go.sum` file is either missing or incomplete. It doesn't contain cryptographic checksums for the following required dependencies:
- `gorm.io/driver/mysql` - MySQL driver for GORM
- `gorm.io/gorm` - GORM ORM library
- `golang.org/x/crypto/bcrypt` - Password hashing
- `github.com/gin-gonic/gin` - HTTP web framework
- `github.com/golang-jwt/jwt/v5` - JWT authentication
- `github.com/joho/godotenv` - Environment variable loader

**Dockerfile Location:** `api/Dockerfile`

---

### 2. Frontend Service (Next.js)

**Status:** ❌ FAILED
**Error Type:** Missing package-lock.json file

**Error Message:**
```
npm error code EUSAGE
npm error
npm error The `npm ci` command can only install with an existing package-lock.json or
npm error npm-shrinkwrap.json with lockfileVersion >= 1. Run an install with npm@5 or
npm error later to generate a package-lock.json file, then try again.
```

**Build Command Failed:**
```bash
RUN npm ci
```

**Exit Code:** 1

**Root Cause:** The `frontend/package-lock.json` file does not exist. The Dockerfile uses `npm ci` (clean install) which requires a lockfile for reproducible builds. The `COPY package.json package-lock.json* ./` command copies package.json but the optional wildcard for package-lock.json* finds nothing.

**Dockerfile Location:** `frontend/Dockerfile`
**Dockerfile Line:** `RUN npm ci` in the `deps` stage

---

### 3. Admin Service (Next.js)

**Status:** ❌ FAILED
**Error Type:** Missing package-lock.json file

**Error Message:**
```
npm error code EUSAGE
npm error
npm error The `npm ci` command can only install with an existing package-lock.json or
npm error npm-shrinkwrap.json with lockfileVersion >= 1. Run an install with npm@5 or
npm error later to generate a package-lock.json file, then try again.
```

**Build Command Failed:**
```bash
RUN npm ci
```

**Exit Code:** 1

**Root Cause:** The `admin/package-lock.json` file does not exist. Identical issue to the frontend service - `npm ci` requires a lockfile that isn't present in the repository.

**Dockerfile Location:** `admin/Dockerfile`
**Dockerfile Line:** `RUN npm ci` in the `deps` stage

---

## Services Not Built

- **MySQL:** Uses official Docker image - no build required (correctly skipped)

---

## Next Steps

To fix these issues, the following actions are required:

### API Service Fix
1. Run `go mod download` or `go mod tidy` to regenerate go.sum with all required dependencies
2. Ensure the go.sum file is committed to the repository
3. Alternatively, modify the Dockerfile to run `go mod download` before the build step

### Frontend Service Fix
1. Navigate to `frontend/` directory
2. Run `npm install` to generate package-lock.json
3. Commit the package-lock.json file to the repository
4. Alternatively, change Dockerfile to use `npm install` instead of `npm ci`

### Admin Service Fix
1. Navigate to `admin/` directory
2. Run `npm install` to generate package-lock.json
3. Commit the package-lock.json file to the repository
4. Alternatively, change Dockerfile to use `npm install` instead of `npm ci`

---

## Environment Details

- **Docker Compose Version:** Logged in build output
- **Base Images Used:**
  - API: `golang:1.23-alpine` (builder), `alpine:latest` (runner)
  - Frontend: `node:20-alpine` (both deps and runner stages)
  - Admin: `node:20-alpine` (both deps and runner stages)
- **Multi-stage Build:** Yes, all services use multi-stage builds
- **Build Context:** Service-specific directories (api/, frontend/, admin/)

---

## Full Build Output

The complete build output has been saved to `build_output.txt` for detailed analysis.
