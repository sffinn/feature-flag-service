# Build
FROM golang:1.27-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/featureflag-service ./cmd/server

# Runtime
FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=build /bin/featureflag-service /app/featureflag-service
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/app/featureflag-service"]
