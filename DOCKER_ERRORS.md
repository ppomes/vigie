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

**Root Cause Analysis:**

The Frontend service build fails due to a **missing package-lock.json file** in the repository. This is a fundamental npm build configuration issue:

1. **Missing package-lock.json File (Primary Issue)**
   - The `frontend/package-lock.json` file does not exist in the repository
   - Confirmed via repository inspection - only package.json is present
   - package-lock.json contains exact dependency versions and integrity hashes for all packages
   - npm's `ci` command is designed for automated environments and requires this lockfile for reproducibility

2. **Dockerfile Build Strategy (Context)**
   - The Dockerfile uses `npm ci` (clean install) instead of `npm install` for production builds
   - This is a deliberate best practice choice for several reasons:
     - **Speed**: `npm ci` is significantly faster than `npm install` (skips dependency resolution)
     - **Reproducibility**: Installs exact versions from lockfile, not semver ranges from package.json
     - **Security**: Validates package integrity using checksums in lockfile
     - **Consistency**: Ensures identical node_modules across all environments (dev, CI, production)
   - The Dockerfile line `COPY package.json package-lock.json* ./` uses an optional wildcard `*`
   - This allows the build to proceed if package-lock.json is missing, but `npm ci` will fail

3. **Why package-lock.json is Critical**
   - **Dependency Resolution**: package.json uses semver ranges (e.g., `^15.1.0`, `^19.0.0`)
   - Without a lockfile, different installs could get different versions (^15.1.0 could resolve to 15.1.0 OR 15.2.0)
   - The lockfile "locks" exact versions, including all transitive dependencies
   - For Next.js projects with 200+ dependencies, this prevents version drift and "works on my machine" issues

4. **Multi-Stage Build Impact**
   - The frontend Dockerfile uses a multi-stage build pattern:
     - Stage 1 (`deps`): Install dependencies using npm ci
     - Stage 2 (`builder`): Copy deps and build the Next.js app
     - Stage 3 (`runner`): Copy build artifacts to minimal production image
   - The failure occurs in the **first stage** (`deps`), preventing any subsequent stages from executing
   - This is a build-time error, not a runtime error

**Dependencies Affected (from package.json):**
- Production:
  - `next: ^15.1.0` - Next.js framework
  - `next-intl: ^3.25.2` - Internationalization library
  - `react: ^19.0.0` - React library
  - `react-dom: ^19.0.0` - React DOM renderer
- Development (17 packages):
  - TypeScript, Testing libraries, Tailwind CSS, Vitest, and more

**Dockerfile Location:** `frontend/Dockerfile`
**Affected Build Stage:** Line 15 - `RUN npm ci` in the `deps` stage

**Proposed Fix Options:**

**Option A: Generate and Commit package-lock.json (Recommended)**
   ```bash
   cd frontend
   npm install  # Generates package-lock.json with all resolved dependencies
   git add package-lock.json
   git commit -m "Add package-lock.json for frontend service"
   ```
   **Benefits:**
   - Ensures reproducible builds across all environments
   - Enables faster `npm ci` installs in Docker and CI/CD
   - Locks dependency versions, preventing unexpected updates
   - Follows npm and Docker best practices
   - No Dockerfile changes needed
   - Expected lockfileVersion: 3 (npm 7+)

**Option B: Modify Dockerfile to Use npm install**
   ```dockerfile
   # Change in deps stage:
   RUN npm install --production=false
   # Or for production only:
   RUN npm install --production
   ```
   **Drawbacks:**
   - Slower builds (re-resolves dependencies every time)
   - Non-reproducible (different builds may get different versions)
   - Defeats the purpose of using `npm ci` in Docker
   - Not recommended for production environments

**Option C: Generate lockfile in Dockerfile**
   ```dockerfile
   # After copying package.json:
   RUN npm install --package-lock-only
   RUN npm ci
   ```
   **Drawbacks:**
   - Lockfile not committed to repository (still non-reproducible)
   - Each build generates potentially different lockfile
   - Adds extra build step and time
   - Not recommended for production

**Recommendation:** Option A is strongly recommended. The lockfile should be generated locally and committed to version control to ensure all team members and CI/CD environments use identical dependency versions.

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

**Root Cause Analysis:**

The Admin service build fails due to a **missing package-lock.json file** in the repository. This issue is identical in nature to the Frontend service failure, as both are Next.js applications using the same Docker build strategy:

1. **Missing package-lock.json File (Primary Issue)**
   - The `admin/package-lock.json` file did not exist in the repository at build time
   - Confirmed via repository inspection - only package.json was present
   - package-lock.json contains exact dependency versions and integrity hashes for all packages
   - npm's `ci` command is designed for automated environments and requires this lockfile for reproducibility

2. **Dockerfile Build Strategy (Context)**
   - The Dockerfile uses `npm ci` (clean install) instead of `npm install` for production builds
   - This is a deliberate best practice choice for several reasons:
     - **Speed**: `npm ci` is significantly faster than `npm install` (skips dependency resolution)
     - **Reproducibility**: Installs exact versions from lockfile, not semver ranges from package.json
     - **Security**: Validates package integrity using checksums in lockfile
     - **Consistency**: Ensures identical node_modules across all environments (dev, CI, production)
   - The Dockerfile line `COPY package.json package-lock.json* ./` uses an optional wildcard `*`
   - This allows the build to proceed if package-lock.json is missing, but `npm ci` will fail

3. **Why package-lock.json is Critical for Admin Service**
   - **Dependency Resolution**: package.json uses semver ranges (e.g., `^15.1.0`, `^19.0.0`)
   - Without a lockfile, different installs could get different versions (^15.1.0 could resolve to 15.1.0 OR 15.2.0)
   - The lockfile "locks" exact versions, including all transitive dependencies
   - The Admin service uses next-intl for internationalization, which adds additional dependency complexity
   - For Next.js projects with 200+ dependencies, this prevents version drift and "works on my machine" issues

4. **Multi-Stage Build Impact**
   - The admin Dockerfile uses a multi-stage build pattern (identical to frontend):
     - Stage 1 (`deps`): Install dependencies using npm ci
     - Stage 2 (`builder`): Copy deps and build the Next.js app with `output: 'standalone'` configuration
     - Stage 3 (`runner`): Copy build artifacts (.next/standalone, .next/static, public) to minimal production image
   - The failure occurs in the **first stage** (`deps`), preventing any subsequent stages from executing
   - This is a build-time error, not a runtime error

5. **Admin Service-Specific Configuration**
   - **Port Configuration**: Admin runs on port 3001 (vs Frontend on 3000)
   - **Standalone Output**: next.config.js specifies `output: 'standalone'` for Docker optimization
   - **Internationalization**: Uses next-intl plugin with custom i18n configuration (./i18n.ts)
   - **Health Check**: Dockerfile includes health check on port 3001 (wget http://localhost:3001)
   - These configurations are correctly set up in Dockerfile and next.config.js, but build fails before reaching runtime

**Dependencies Affected (from admin/package.json):**
- Production:
  - `next: ^15.1.0` - Next.js framework
  - `next-intl: ^3.25.2` - Internationalization library (critical for admin UI localization)
  - `react: ^19.0.0` - React library
  - `react-dom: ^19.0.0` - React DOM renderer
- Development (17 packages):
  - TypeScript, Testing libraries (@testing-library/react, @testing-library/jest-dom)
  - Tailwind CSS (autoprefixer, postcss, tailwindcss)
  - Vitest testing framework (@vitest/ui, jsdom, vitest)
  - Build tools and type definitions

**Dockerfile Location:** `admin/Dockerfile`
**Affected Build Stage:** Line 15 - `RUN npm ci` in the `deps` stage

**Proposed Fix Options:**

**Option A: Generate and Commit package-lock.json (Recommended)**
   ```bash
   cd admin
   npm install  # Generates package-lock.json with all resolved dependencies
   git add package-lock.json
   git commit -m "Add package-lock.json for admin service"
   ```
   **Benefits:**
   - Ensures reproducible builds across all environments
   - Enables faster `npm ci` installs in Docker and CI/CD
   - Locks dependency versions for next-intl and all Next.js 15+ dependencies
   - Follows npm and Docker best practices
   - No Dockerfile changes needed
   - Expected lockfileVersion: 3 (npm 7+)

**Option B: Modify Dockerfile to Use npm install**
   ```dockerfile
   # Change in deps stage:
   RUN npm install --production=false
   # Or for production only:
   RUN npm install --production
   ```
   **Drawbacks:**
   - Slower builds (re-resolves dependencies every time)
   - Non-reproducible (different builds may get different versions)
   - Defeats the purpose of using `npm ci` in Docker
   - Not recommended for production environments
   - Could break due to version drift in next-intl or Next.js 15

**Option C: Generate lockfile in Dockerfile**
   ```dockerfile
   # After copying package.json:
   RUN npm install --package-lock-only
   RUN npm ci
   ```
   **Drawbacks:**
   - Lockfile not committed to repository (still non-reproducible)
   - Each build generates potentially different lockfile
   - Adds extra build step and time
   - Not recommended for production

**Recommendation:** Option A is strongly recommended. The lockfile should be generated locally and committed to version control to ensure all team members and CI/CD environments use identical dependency versions. This is especially critical for the Admin service, which uses internationalization (next-intl) and serves on a different port (3001), requiring stable and reproducible builds.

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
