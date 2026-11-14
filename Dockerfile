FROM golang:1.26-alpine AS builder

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o findit-api .

FROM alpine:3.24

RUN adduser -D -g '' appuser && mkdir -p /app/logs && chown -R appuser:appuser /app

WORKDIR /app

COPY --from=builder /build/findit-api /app/findit-api

USER appuser

EXPOSE 3000

ENTRYPOINT ["/app/findit-api"]

CMD ["-role=all"]

