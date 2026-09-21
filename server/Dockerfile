FROM golang:1.26-bookworm AS build

WORKDIR /src
COPY go.mod go.sum ./
COPY agent/go.mod ./agent/go.mod
RUN go mod download
COPY . .

ENV CGO_ENABLED=0
RUN go build -trimpath -ldflags="-s -w" -o /out/spendable-today ./cmd/api

FROM debian:bookworm-slim

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates wget \
    && rm -rf /var/lib/apt/lists/* \
    && useradd --system --create-home --uid 10001 appuser

WORKDIR /app
COPY --from=build /out/spendable-today /app/spendable-today

ENV API_ADDR=0.0.0.0:8080 \
    USE_MOCK_LLM=true

USER appuser
EXPOSE 8080
ENTRYPOINT ["/app/spendable-today"]
