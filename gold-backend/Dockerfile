FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o /gold ./cmd/gold

FROM alpine:3.20
COPY --from=builder /gold /gold
EXPOSE 8080
CMD ["/gold"]
