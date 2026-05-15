FROM alpine:3.20

ARG TARGETARCH
RUN apk add --no-cache tzdata ca-certificates

WORKDIR /app
COPY --chmod=0755 dist/server-linux-${TARGETARCH} /app/server

ENV GIN_MODE=release

EXPOSE 8080
ENTRYPOINT ["/app/server"]
