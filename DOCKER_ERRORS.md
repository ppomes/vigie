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

**Root Cause Analysis:**

The API service build fails due to a **missing go.sum file** combined with an **incomplete dependency declaration** in go.mod. This is a three-part issue:

1. **Missing go.sum File (Primary Issue)**
   - The `api/go.sum` file does not exist in the repository
   - Confirmed via `ls -la api/` - only go.mod is present
   - go.sum contains cryptographic checksums for all dependencies and their transitive dependencies
   - Go's build system requires go.sum for security and reproducibility

2. **Incomplete go.mod Declaration (Secondary Issue)**
   - The package `golang.org/x/crypto/bcrypt` is directly imported in `api/internal/models/user.go:6`
   - However, it is NOT declared in the `require` section of `api/go.mod`
   - This is a direct dependency (not transitive) and must be explicitly declared
   - Current go.mod only lists 6 packages, missing this critical crypto dependency

3. **Dockerfile Build Flow Limitation (Contributing Factor)**
   - The Dockerfile copies go.mod and attempts to copy go.sum with optional wildcard: `COPY go.sum* ./`
   - Then runs `RUN go mod download` expecting it to create go.sum
   - **However**: `go mod download` only downloads modules listed in go.mod; it does NOT create a complete go.sum file with all checksums
   - The go.sum file is only created/updated when running: `go mod tidy`, `go build`, `go test`, or `go get`
   - When `go build` runs in the Dockerfile (line 25), it requires go.sum to already exist and be complete

**Missing Dependencies Identified:**
- `gorm.io/driver/mysql` - MySQL driver for GORM (declared in go.mod)
- `gorm.io/gorm` - GORM ORM library (declared in go.mod)
- `golang.org/x/crypto/bcrypt` - Password hashing (**NOT in go.mod, directly used in code**)
- `github.com/gin-gonic/gin` - HTTP web framework (declared in go.mod)
- `github.com/golang-jwt/jwt/v5` - JWT authentication (declared in go.mod)
- `github.com/joho/godotenv` - Environment variable loader (declared in go.mod)

**Dockerfile Location:** `api/Dockerfile`
**Affected Build Stage:** Line 25 - `RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o api ./cmd/api/main.go`

**Proposed Fix Options:**

**Option A: Generate and Commit go.sum (Recommended)**
   ```bash
   cd api
   go mod tidy  # This will update go.mod with missing deps and create go.sum
   git add go.sum go.mod
   git commit -m "Add go.sum and fix missing golang.org/x/crypto dependency"
   ```
   - Ensures reproducible builds
   - Follows Go best practices
   - No Dockerfile changes needed

**Option B: Modify Dockerfile to Generate go.sum**
   ```dockerfile
   # After copying go.mod, add:
   RUN go mod tidy
   # Or after copying source code, before build:
   RUN go mod download && go mod verify
   ```
   - Less secure (checksums not verified before download)
   - Builds may not be reproducible
   - Not recommended for production

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
