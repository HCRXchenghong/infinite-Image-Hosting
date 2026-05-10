FROM golang:1.25-bookworm AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/yuexiang-api ./cmd/api
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/yuexiang-worker ./cmd/worker

FROM debian:bookworm-slim

RUN apt-get update \
  && apt-get install -y --no-install-recommends ca-certificates curl libvips-tools \
  && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY --from=build /out/yuexiang-api /app/yuexiang-api
COPY --from=build /out/yuexiang-worker /app/yuexiang-worker
COPY migrations /app/migrations
COPY docs /app/docs

ENV HTTP_ADDR=:8080
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=5s --retries=3 CMD curl -fsS http://127.0.0.1:8080/readyz >/dev/null || exit 1

CMD ["/app/yuexiang-api"]
