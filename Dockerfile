FROM alpine:3.20

ARG TARGETARCH
RUN apk add --no-cache tzdata ca-certificates

WORKDIR /app
COPY --chmod=0755 dist/server-linux-${TARGETARCH} /app/server

ENV GIN_MODE=release

EXPOSE 8081

HEALTHCHECK --interval=5m --timeout=5s --start-period=30s --start-interval=5s --retries=3 \
	CMD wget -q --spider "http://127.0.0.1:${SERVER_PORT:-8081}/healthz" || exit 1

ENTRYPOINT ["/app/server"]
