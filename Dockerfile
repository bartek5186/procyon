FROM alpine:3.20

WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata wget && \
    mkdir -p /app/config /app/static /app/log

COPY build/procyon-server /app/procyon-server
COPY static /app/static

EXPOSE 8081

HEALTHCHECK --interval=30s --timeout=3s --start-period=20s --retries=3 \
  CMD wget -q -O /dev/null http://127.0.0.1:8081/healthz || exit 1

CMD ["/app/procyon-server"]
