# syntax=docker/dockerfile:1

FROM golang:1.27.1-bookworm AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /bin/feature-flag-service ./cmd/feature-flag-service

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=build /bin/feature-flag-service /app/feature-flag-service
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/app/feature-flag-service"]
