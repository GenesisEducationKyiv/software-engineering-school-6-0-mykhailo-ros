FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY gen/ gen/
COPY subscription-service/go.mod subscription-service/go.sum subscription-service/
WORKDIR /app/subscription-service
RUN go mod download
WORKDIR /app
COPY subscription-service/ subscription-service/
WORKDIR /app/subscription-service
RUN go build -o main ./cmd/main.go

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/subscription-service/main .
COPY subscription-service/migrations/ migrations/
COPY subscription-service/static/ static/
COPY subscription-service/swagger.yaml .
EXPOSE 8080
EXPOSE 9090
CMD ["./main"]
