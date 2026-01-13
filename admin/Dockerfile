# Multi-stage build for Next.js Admin service

# Stage 1: Dependencies
FROM node:20-alpine AS deps

# Install dependencies for node-gyp (if needed)
RUN apk add --no-cache libc6-compat

WORKDIR /app

# Copy package files for dependency installation
COPY package.json package-lock.json* ./

# Install dependencies using npm ci for reproducible builds
# Use --legacy-peer-deps due to React 19 and @testing-library/react@14 peer dependency conflicts
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

# Create public directory if it doesn't exist (prevents COPY errors)
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
ENV HOSTNAME=0.0.0.0

# Copy Next.js build output
# Using regular build (not standalone) due to Next.js 15.5.9 route group bug
COPY --from=builder --chown=nodeuser:nodegroup /app/.next ./.next
COPY --from=builder --chown=nodeuser:nodegroup /app/public ./public
COPY --from=builder --chown=nodeuser:nodegroup /app/package.json ./package.json

# Copy only production dependencies
COPY --from=builder --chown=nodeuser:nodegroup /app/node_modules ./node_modules

# Copy necessary source files for Next.js runtime
COPY --from=builder --chown=nodeuser:nodegroup /app/next.config.js ./next.config.js
COPY --from=builder --chown=nodeuser:nodegroup /app/i18n.ts ./i18n.ts
COPY --from=builder --chown=nodeuser:nodegroup /app/middleware.ts ./middleware.ts
COPY --from=builder --chown=nodeuser:nodegroup /app/messages ./messages

# Copy environment example file as reference
COPY --from=builder --chown=nodeuser:nodegroup /app/.env.local.example* ./

# Switch to non-root user
USER nodeuser

# Expose Next.js port (admin runs on 3001)
EXPOSE 3001

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:3001 || exit 1

# Start the Next.js server using npm start
# This uses the regular Next.js production server
CMD ["npm", "start"]
