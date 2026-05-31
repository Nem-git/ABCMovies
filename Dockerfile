############ Image de base pour Go ############
FROM golang:1.26.3-alpine3.23 AS golang-base

WORKDIR /usr/src/app

# Download dependencies
# src/go.sum
COPY src/go.mod ./
RUN go mod download

# Copy code to container
COPY src/ ./

############ Dev ############
FROM golang-base AS dev

COPY --from=cosmtrek/air:v1.65.3 /go/bin/air /go/bin/air

# Start hot reloading server
EXPOSE 80
CMD ["air", "-c", "./config/.air.toml"]

############ Testing ############
FROM golang-base AS testing

CMD ["go", "test", "-v", "./..."]

########### Prod ############
FROM golang-base AS builder-prod
# Build prod app
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -trimpath -v -o /go/bin/app ./cmd

# Create minimal /etc/passwd
RUN echo "appuser:x:10001:10001:App User:/:/sbin/nologin" > /etc/minimal-passwd

FROM golang-base AS certs
RUN apk add --no-cache ca-certificates

FROM scratch AS prod

# Copier le binaire compilé depuis l'étape de build + certificats pour les requetes HTTPS
COPY --from=builder-prod /go/bin/app /go/bin/app
COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Create and set nonroot user
COPY --from=builder-prod /etc/minimal-passwd /etc/passwd
USER appuser

# Start app
EXPOSE 80
ENTRYPOINT ["/go/bin/app"]
