FROM golang:1.27.4-alpine AS builder

WORKDIR /app

COPY go.mod ./go.mod
COPY go.sum ./go.sum
RUN go mod download

COPY . .

RUN go build -o door-opener .

FROM alpine:latest

WORKDIR /app

COPY --from=builder /app/door-opener .
COPY templates ./templates

EXPOSE 8881

CMD ["./door-opener"]
