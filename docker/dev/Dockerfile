# syntax=docker/dockerfile:1

FROM golang:1.24-bookworm
WORKDIR /app

RUN apt-get update \
    && apt-get install -y --no-install-recommends git ca-certificates \
    && rm -rf /var/lib/apt/lists/*

RUN go install github.com/air-verse/air@v1.61.7

COPY go.mod go.sum ./
RUN go mod download

COPY docker/dev/.air.toml /app/docker/dev/.air.toml

EXPOSE 8080

CMD ["air", "-c", "docker/dev/.air.toml"]
