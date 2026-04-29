FROM alpine:3.20

WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata wget && \
    mkdir -p /app/config /app/static /app/log

COPY build/procyon-server /app/procyon-server
COPY static /app/static

EXPOSE 8080 8081 8082

HEALTHCHECK --interval=30s --timeout=3s --start-period=20s --retries=3 \
  CMD wget -q -O /dev/null http://127.0.0.1:8080/healthz || exit 1

CMD ["/app/procyon-server"]
