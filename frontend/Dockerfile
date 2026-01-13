# Multi-stage build for Next.js Frontend service

# Stage 1: Dependencies
FROM node:20-alpine AS deps

# Install dependencies for node-gyp (if needed)
RUN apk add --no-cache libc6-compat

WORKDIR /app

# Copy package files for dependency installation
COPY package.json package-lock.json* ./

# Install dependencies using npm ci for reproducible builds
# Use --legacy-peer-deps due to React 19 and @testing-library/react@14 peer dependency conflict
RUN npm ci --legacy-peer-deps

# Stage 2: Builder
FROM node:20-alpine AS builder

WORKDIR /app

# Copy dependencies from deps stage
COPY --from=deps /app/node_modules ./node_modules

# Copy application source
COPY . .

# Set environment variable for API URL (can be overridden at runtime)
ENV NEXT_TELEMETRY_DISABLED=1

# Create public directory if it doesn't exist (needed for COPY in runner stage)
RUN mkdir -p public

# Build the Next.js application
# This will create .next/standalone directory due to output: 'standalone' in next.config.js
RUN npm run build

# Stage 3: Runner (Production)
FROM node:20-alpine AS runner

WORKDIR /app

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates

# Create non-root user for security
RUN addgroup -S nodegroup && adduser -S nodeuser -G nodegroup

# Set to production environment
ENV NODE_ENV=production
ENV NEXT_TELEMETRY_DISABLED=1

# Copy standalone output from builder
# Next.js standalone output includes minimal files needed to run
COPY --from=builder --chown=nodeuser:nodegroup /app/.next/standalone ./
COPY --from=builder --chown=nodeuser:nodegroup /app/.next/static ./.next/static
COPY --from=builder --chown=nodeuser:nodegroup /app/public ./public

# Copy environment example file as reference
COPY --from=builder --chown=nodeuser:nodegroup /app/.env.local.example* ./

# Switch to non-root user
USER nodeuser

# Expose Next.js port
EXPOSE 3000

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:3000 || exit 1

# Start the Next.js server
# The standalone server is in server.js
CMD ["node", "server.js"]
