# syntax=docker/dockerfile:1

# --- build -----------------------------------------------------------------
FROM docker.io/library/golang:1.27-alpine AS build

WORKDIR /src

# Cache dependencies separately from sources.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Static binary; -trimpath keeps paths out of the artifact.
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/groupify-api ./cmd/api \
 && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/groupify-seed ./cmd/seed

# --- runtime ---------------------------------------------------------------
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /out/groupify-api /usr/local/bin/groupify-api
COPY --from=build /out/groupify-seed /usr/local/bin/groupify-seed

USER nonroot:nonroot
EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/groupify-api"]
