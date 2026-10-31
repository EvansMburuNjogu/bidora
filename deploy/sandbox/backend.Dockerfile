FROM golang:1.27.2-bookworm AS build
WORKDIR /src
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=0 go build -trimpath -o /out/identity-api ./cmd/identity-api && \
    CGO_ENABLED=0 go build -trimpath -o /out/migrate ./cmd/migrate && \
    CGO_ENABLED=0 go build -trimpath -o /out/grant-runtime ./cmd/grant-runtime
FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates curl && rm -rf /var/lib/apt/lists/* && useradd --uid 10001 --create-home bidora
COPY --from=build /out/ /usr/local/bin/
USER bidora
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/identity-api"]
