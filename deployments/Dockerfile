# Build with CGO for github.com/mattn/go-sqlite3
FROM golang:1.26-bookworm AS builder

WORKDIR /src
ENV CGO_ENABLED=1
ENV GOTOOLCHAIN=auto

RUN apt-get update && apt-get install -y --no-install-recommends \
	gcc libc6-dev libsqlite3-dev \
	&& rm -rf /var/lib/apt/lists/*

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -trimpath -ldflags="-s -w" -o /out/graft ./cmd/graft

FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
	ca-certificates libsqlite3-0 wget \
	&& rm -rf /var/lib/apt/lists/*

RUN useradd --uid 65532 --system --no-create-home --shell /usr/sbin/nologin graft \
	&& mkdir -p /data && chown graft:graft /data

COPY --from=builder /out/graft /usr/local/bin/graft

USER graft:graft
WORKDIR /data

ENV DB_PATH=/data/rules.db
EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/graft"]
