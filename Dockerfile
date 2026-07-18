############ Image de base pour Go ############
FROM golang:1.26.3-alpine3.23 AS golang-base

WORKDIR /usr/src/app

# Install build tools
RUN apk add --no-cache make curl gcc musl-dev

# Installs Tailwind CSS CLI
RUN curl -fsSL https://github.com/tailwindlabs/tailwindcss/releases/download/v4.3.1/tailwindcss-linux-x64-musl -o tailwindcss
RUN chmod +x tailwindcss

# Installs ESBuild
RUN curl -fsSL https://esbuild.github.io/dl/v0.28.1 | sh

# Download dependencies
# src/go.sum
COPY go.mod go.sum ./
RUN go mod download

# Cache layer: generate ogen code (only invalidates when API spec changes)
COPY api ./api
COPY generate.go .ogen.yaml ./
RUN go generate ./...

# Copy remaining source code and regenerate fast assets
COPY . ./
RUN make generate/templ generate/css generate/js

############ Dev ############
FROM golang-base AS dev

COPY --from=cosmtrek/air:v1.65.3 /go/bin/air /go/bin/air

# Start hot reloading server
EXPOSE 80
CMD make live

############ Testing ############
FROM golang-base AS testing

CMD make test && make vet

########### Prod ############
FROM golang-base AS builder-prod
# Generate assets and build prod app
RUN make generate/templ generate/css
RUN make build

# Create minimal /etc/passwd
RUN echo "appuser:x:10001:10001:App User:/:/sbin/nologin" > /etc/minimal-passwd

FROM golang-base AS certs
RUN apk add --no-cache ca-certificates

FROM scratch AS prod

# Copier le binaire compilé depuis l'étape de build + certificats pour les requetes HTTPS
COPY --from=builder-prod /usr/src/app/bin/abcmovies /go/bin/app
COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Create and set nonroot user
COPY --from=builder-prod /etc/minimal-passwd /etc/passwd
USER appuser

# Start app
EXPOSE 80
ENTRYPOINT ["/go/bin/app"]
