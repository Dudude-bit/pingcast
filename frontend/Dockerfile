FROM node:20-alpine AS base
# Install pnpm directly instead of through corepack: corepack would
# fetch the pnpm binary from the npm registry on every container start
# (package.json has no "packageManager" field) and then block the build
# in environments that can't reach the registry (e.g. Dokploy builds
# hitting a Cloudflare-routed npm host timeout).
RUN npm install -g pnpm@10.33.0
WORKDIR /app

FROM base AS deps
COPY frontend/package.json frontend/pnpm-lock.yaml ./
RUN --mount=type=cache,target=/root/.local/share/pnpm/store \
    pnpm install --frozen-lockfile

FROM base AS builder
ARG INTERNAL_API_URL=http://api:8080/api
ENV INTERNAL_API_URL=$INTERNAL_API_URL
COPY --from=deps /app/node_modules ./node_modules
COPY frontend/ ./
# Publish the OpenAPI spec as a static asset so /docs/api can load it
# without needing a runtime API call. The spec is the source of truth
# for both the Go handlers and the TS types — serving it directly keeps
# docs in lockstep with the API surface.
COPY api/openapi.yaml ./public/openapi.yaml
RUN pnpm build

FROM base AS runner
ENV NODE_ENV=production
RUN addgroup --system --gid 1001 nodejs \
 && adduser --system --uid 1001 nextjs
COPY --from=builder --chown=nextjs:nodejs /app/.next/standalone ./
COPY --from=builder --chown=nextjs:nodejs /app/.next/static ./.next/static
COPY --from=builder --chown=nextjs:nodejs /app/public ./public
USER nextjs
EXPOSE 3000
ENV PORT=3000 HOSTNAME=0.0.0.0
CMD ["node", "server.js"]
