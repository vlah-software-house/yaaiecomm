# Stage 1: Build the Go binary
FROM golang:1.23-alpine AS build

RUN apk add --no-cache git ca-certificates

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o /bin/forgecommerce ./cmd/server

# Stage 2: Minimal runtime image
FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata

RUN addgroup -S app && adduser -S app -G app

WORKDIR /app

COPY --from=build /bin/forgecommerce ./forgecommerce
COPY --from=build /src/internal/database/migrations ./migrations
COPY --from=build /src/templates ./templates
COPY --from=build /src/static ./static

RUN chown -R app:app /app
USER app

EXPOSE 8080 8081

ENTRYPOINT ["./forgecommerce"]
