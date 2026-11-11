# syntax=docker/dockerfile:1

# One image, one origin: the Go API serves the built SPA from /app/static, so
# session cookies stay first-party and no CORS or reverse-proxy rewrite is
# needed.

# --- frontend ---------------------------------------------------------------
FROM docker.io/library/node:24-alpine AS web

WORKDIR /web

# Cache the dependency tree separately from the sources.
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci

COPY frontend/ ./
RUN npm run build

# --- backend ---------------------------------------------------------------
FROM docker.io/library/golang:1.27-alpine AS build

WORKDIR /src

COPY backend/go.mod backend/go.sum ./
RUN go mod download

COPY backend/ ./

RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/groupify-api ./cmd/api \
 && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/groupify-seed ./cmd/seed

# --- runtime ---------------------------------------------------------------
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /out/groupify-api /usr/local/bin/groupify-api
COPY --from=build /out/groupify-seed /usr/local/bin/groupify-seed
COPY --from=web /web/dist /app/static

# Serving the SPA is opt-in, so nothing is served by accident in tests.
ENV STATIC_DIR=/app/static

USER nonroot:nonroot
EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/groupify-api"]
