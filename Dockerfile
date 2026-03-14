FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o api ./cmd/api

FROM alpine:3.21
RUN apk --no-cache upgrade && apk --no-cache add ca-certificates
RUN addgroup -S appgroup && adduser -S appuser -G appgroup
WORKDIR /app
COPY --from=builder /app/api .
COPY migrations/ ./migrations/
USER appuser
EXPOSE 8080
CMD ["./api"]
