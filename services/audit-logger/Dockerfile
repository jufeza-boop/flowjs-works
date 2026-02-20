FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o audit-logger ./cmd/main.go

FROM alpine:3.19
RUN apk --no-cache add ca-certificates tzdata
WORKDIR /app
COPY --from=builder /app/audit-logger .
EXPOSE 8080
ENTRYPOINT ["./audit-logger"]
