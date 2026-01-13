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

## Docker Compose Configuration Issues

While the build failures prevent testing runtime behavior, analysis of the `docker-compose.yml` file reveals several configuration issues that will cause **runtime failures** once the build errors are resolved.

### Issue 1: Incorrect API URL for Frontend/Admin Services ⚠️ CRITICAL

**Status:** ❌ RUNTIME FAILURE (after build is fixed)
**Impact:** Frontend and Admin services cannot communicate with API

**Problem:**
- **Lines 94, 121:** `NEXT_PUBLIC_API_URL: ${NEXT_PUBLIC_API_URL:-http://localhost:8080/api/v1}`
- Default value uses `localhost:8080` which does NOT work inside Docker containers
- `NEXT_PUBLIC_API_URL` is baked into the Next.js build at **build time** (not runtime)
- Inside containers, `localhost` refers to the container itself, not the API service

**Why This Fails:**
1. Frontend container starts and tries to fetch from `http://localhost:8080/api/v1`
2. Port 8080 is not exposed inside the frontend container (only in the API container)
3. Frontend cannot reach the API - all API calls fail with connection errors
4. Same issue affects Admin service on port 3001

**Evidence from .env.example:**
```bash
# Lines 55-58 of .env.example:
NEXT_PUBLIC_API_URL=http://localhost:8080/api/v1

# For Docker Compose, use:
# NEXT_PUBLIC_API_URL=http://api:8080/api/v1
```
The .env.example file explicitly documents this requirement but the docker-compose.yml doesn't follow it.

**Network Configuration Context:**
- All services are on `statuspage-network` (bridge network)
- Services can reference each other by **service name** (e.g., `api`, `mysql`)
- The API service is named `api` (line 33) and exposes port 8080 (line 70)
- Correct URL should be: `http://api:8080/api/v1`

**Root Cause:**
The docker-compose.yml inherits the .env.example default value, which is designed for **local development outside Docker** (where the API runs on the host's localhost:8080). This configuration is fundamentally incompatible with Docker Compose networking.

**Proposed Fix:**
```yaml
# Lines 94, 121 - Update both frontend and admin:
environment:
  NODE_ENV: ${NODE_ENV:-production}
  NEXT_PUBLIC_API_URL: ${NEXT_PUBLIC_API_URL:-http://api:8080/api/v1}  # Changed from localhost
  NEXT_TELEMETRY_DISABLED: 1
```

**Alternative Fix (requires user to update .env):**
Document in README that users must set `NEXT_PUBLIC_API_URL=http://api:8080/api/v1` in their .env file before running docker-compose.

---

### Issue 2: API Health Check Endpoint Dependency

**Status:** ✅ VERIFIED (endpoint exists)
**Impact:** Startup sequence depends on this endpoint

**Configuration:**
- **Line 74:** `test: ["CMD", "wget", "--quiet", "--tries=1", "--spider", "http://localhost:8080/api/v1/status"]`
- Frontend and Admin services depend on `api: condition: service_healthy` (lines 90-91, 117-118)
- This creates a startup dependency chain: MySQL → API → Frontend/Admin

**Verification:**
The `/api/v1/status` endpoint **is correctly implemented** in `api/cmd/api/main.go:60`:
```go
public.GET("/status", handlers.GetStatus(db))
```

**Startup Sequence:**
1. MySQL container starts and becomes healthy (30 second start_period)
2. API container waits for MySQL health check to pass
3. API container starts and becomes healthy (40 second start_period)
4. Frontend and Admin containers wait for API health check to pass
5. Frontend and Admin containers start

**Potential Issues:**
- If the status endpoint fails (e.g., database connection issues), the API never becomes healthy
- Frontend and Admin will never start if API doesn't become healthy
- Total startup time: MySQL (30s) + API (40s) + Frontend/Admin (40s) = ~110 seconds minimum
- Health check failures will be logged but may not be immediately visible to users

**Recommendation:**
This configuration is **correct as-is** for production deployments. The health check ensures services only start when dependencies are ready. No changes needed, but startup time should be documented.

---

### Issue 3: Migration Strategy and Volume Mount

**Status:** ⚠️ DUAL MIGRATION SYSTEM (may cause conflicts)
**Impact:** Database migrations may run twice or conflict

**Configuration:**
- **Line 20:** `- ./api/migrations:/docker-entrypoint-initdb.d:ro`
- MySQL container mounts migrations directory for automatic initialization

**Migration Implementation Analysis:**
The system uses **two migration approaches simultaneously**:

1. **MySQL Container Init (docker-entrypoint-initdb.d):**
   - SQL file: `api/migrations/001_initial_schema.sql` (6.3KB)
   - Runs automatically when MySQL container first starts
   - Only executes on **empty database** (first run only)
   - Creates schema using SQL DDL statements

2. **GORM AutoMigrate (Application Startup):**
   - Code: `api/cmd/api/main.go:30-38`
   - Runs every time the API application starts
   - Migrates 5 models: User, Component, Incident, Subscriber, Translation
   - Uses GORM's schema migration logic (may differ from SQL)

**Potential Conflicts:**
- If SQL migration creates tables differently than GORM expects, AutoMigrate may try to modify them
- GORM AutoMigrate may add/remove columns that differ from SQL schema
- Two sources of truth for schema definition

**Current Behavior:**
- **First startup (clean database):**
  1. MySQL runs 001_initial_schema.sql → creates tables
  2. API starts → GORM AutoMigrate checks tables → may modify them if differences exist

- **Subsequent startups (existing database):**
  1. MySQL skips migrations (database not empty)
  2. API starts → GORM AutoMigrate runs → updates schema if models changed

**Recommendation:**
Choose **one** migration strategy:

**Option A: Use Only GORM AutoMigrate**
```yaml
# Remove this line from docker-compose.yml:
# - ./api/migrations:/docker-entrypoint-initdb.d:ro
```
Pros: Single source of truth (Go models), simpler maintenance
Cons: Loses SQL migration history, harder to review schema changes

**Option B: Use Only SQL Migrations + Disable AutoMigrate**
Remove AutoMigrate from main.go and manage all migrations as SQL files.
Pros: Explicit schema control, migration history, easier to review
Cons: Must manually sync Go models with SQL, more maintenance

**Option C: Keep Current Setup (if schemas match exactly)**
Only viable if `001_initial_schema.sql` produces identical schema to GORM AutoMigrate.
Requires verification and careful maintenance to keep them in sync.

---

### Issue 4: Security Configuration Issues

**Status:** ⚠️ INSECURE DEFAULTS (development-only)
**Impact:** Severe security risk if deployed to production

**Issues Identified:**

1. **Weak JWT Secret (Line 55):**
   ```yaml
   JWT_SECRET_KEY: ${JWT_SECRET_KEY:-your-secret-key-change-in-production}
   ```
   - Default value is a placeholder string, not a secure secret
   - If user doesn't set environment variable, this insecure default is used
   - JWT tokens can be forged if this secret is known
   - Should use a strong random 32+ character secret

2. **Weak Database Credentials (Lines 46-47):**
   ```yaml
   DB_USER: ${DB_USER:-root}
   DB_PASSWORD: ${DB_PASSWORD:-root}
   ```
   - Both default to 'root'/'root' (well-known credentials)
   - Root access exposed to application (violates least privilege principle)
   - No separate user for application vs. database administration

3. **Debug Mode Enabled (Line 52):**
   ```yaml
   GIN_MODE: ${GIN_MODE:-debug}
   ```
   - Defaults to debug mode, which exposes detailed error messages
   - Should be 'release' in production for performance and security
   - Debug mode logs sensitive information

4. **Exposed Database Port (Line 17):**
   ```yaml
   ports:
     - "${DB_PORT:-3306}:3306"
   ```
   - MySQL port exposed to host network
   - Not necessary if only API needs database access (both on same network)
   - Increases attack surface

5. **API URL for Email Links (Line 68):**
   ```yaml
   API_URL: ${API_URL:-http://localhost:3000}
   ```
   - Used in email links (password reset, confirmation, etc.)
   - Defaults to localhost, which won't work for email recipients
   - Should be set to actual production domain

**Recommendations:**

**For Development (Current State):**
- Add prominent warning in README about insecure defaults
- Document that .env must be configured before deployment
- Consider adding a startup warning if JWT_SECRET_KEY == default value

**For Production:**
```yaml
# Required .env settings for production:
JWT_SECRET_KEY=<generated-with-openssl-rand-base64-32>
DB_USER=statuspage_app  # Non-root user
DB_PASSWORD=<strong-random-password>
GIN_MODE=release
API_URL=https://status.yourdomain.com
NEXT_PUBLIC_API_URL=http://api:8080/api/v1

# Remove database port exposure:
# ports:
#   - "${DB_PORT:-3306}:3306"
```

**Security Hardening Steps:**
1. Create separate MySQL user for application (not root)
2. Generate strong JWT secret (32+ bytes random)
3. Set GIN_MODE=release for production
4. Remove database port exposure if not needed
5. Configure API_URL to actual production domain
6. Use HTTPS for all external URLs
7. Consider adding rate limiting middleware
8. Add database connection encryption (TLS)

---

### Issue 5: Resource and Scaling Considerations

**Status:** ⚠️ NOT PRODUCTION-READY (missing resource limits)
**Impact:** Containers can consume unlimited host resources

**Missing Configurations:**

1. **No Resource Limits:**
   - No CPU limits (`cpus: '0.50'` or `cpu_shares`)
   - No memory limits (`mem_limit: 512m` or `memory: '512m'`)
   - Containers can consume all host resources
   - Risk of one service starving others

2. **No Restart Policies for Failures:**
   - All services use `restart: unless-stopped`
   - This restarts on crash but not on health check failure
   - Consider `restart: on-failure` with max retries

3. **No Logging Configuration:**
   - No log rotation or size limits
   - Logs can fill disk space over time
   - Consider adding logging driver configuration

**Recommended Production Configuration:**
```yaml
# Example for API service:
api:
  # ... existing config ...
  deploy:
    resources:
      limits:
        cpus: '1.0'
        memory: 512M
      reservations:
        cpus: '0.25'
        memory: 128M
  logging:
    driver: "json-file"
    options:
      max-size: "10m"
      max-file: "3"
```

---

### Issue 6: Container Port vs Service Port Configuration

**Status:** ⚠️ POTENTIAL CONFUSION (mismatched port environment variables)
**Impact:** Port configuration may not work as expected in all scenarios

**Issue:**
- **Frontend service (lines 96-97):**
  ```yaml
  ports:
    - "${FRONTEND_PORT:-3000}:3000"
  ```
  - Environment variable `FRONTEND_PORT` controls **host port** (left side)
  - Container always listens on **port 3000** (right side, hardcoded)
  - If user sets `FRONTEND_PORT=3001`, host port changes but container still uses 3000

- **Admin service (lines 123-124):**
  ```yaml
  ports:
    - "${ADMIN_PORT:-3001}:3001"
  ```
  - Same pattern: `ADMIN_PORT` controls host port, not container port
  - Admin container hardcoded to port 3001

**Why This Matters:**
- The Next.js applications must be configured to listen on the container port (right side)
- If this doesn't match the Dockerfile CMD or next.config.js, the service won't respond
- Health checks target the container port (e.g., `http://localhost:3001` on line 128)

**Verification Needed:**
Check that Dockerfile CMD and next.config.js specify correct ports:
- Frontend: Must listen on port 3000 (container port)
- Admin: Must listen on port 3001 (container port)

**Current Configuration Assessment:**
This is likely **correct** if:
- `frontend/Dockerfile` CMD uses `PORT=3000` or hardcodes 3000
- `admin/Dockerfile` CMD uses `PORT=3001` or hardcodes 3001
- Both Dockerfiles have been verified (not shown in this analysis)

**Recommendation:**
Document in README that `FRONTEND_PORT` and `ADMIN_PORT` only control **host-side port mapping**, not the port the application listens on inside the container.

---

### Summary of docker-compose.yml Issues

| Issue | Severity | Impact | Status |
|-------|----------|--------|--------|
| API URL (localhost vs service name) | 🔴 Critical | Frontend/Admin cannot reach API | Must Fix |
| Health Check Endpoint | 🟢 OK | Dependency chain works correctly | Verified |
| Dual Migration System | 🟡 Warning | Potential schema conflicts | Review |
| Insecure Defaults | 🔴 Critical | Security vulnerabilities in production | Must Fix |
| No Resource Limits | 🟡 Warning | Resource exhaustion possible | Production Concern |
| Port Configuration | 🟡 Warning | Potential confusion | Document |

**Priority Fix Order:**
1. **Critical:** Update `NEXT_PUBLIC_API_URL` default to `http://api:8080/api/v1`
2. **Critical:** Document security requirements and force .env configuration for production
3. **Important:** Choose and document migration strategy (SQL vs GORM)
4. **Important:** Add resource limits for production deployments
5. **Nice-to-have:** Add comprehensive README with port mapping explanation

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
