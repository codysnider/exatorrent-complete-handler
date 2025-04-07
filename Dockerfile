FROM golang:1.24-alpine AS builder

WORKDIR /src
COPY main.go .
COPY go.mod .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /app

FROM scratch

COPY --from=builder /app /app

ENTRYPOINT ["/app"]
