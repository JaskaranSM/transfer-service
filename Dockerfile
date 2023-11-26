FROM ubuntu:22.04

RUN apt-get update && apt-get install ca-certificates bash curl coreutils tzdata -y

WORKDIR /app
COPY transfer-service .
CMD ["./transfer-service"]

HEALTHCHECK --interval=30s --timeout=5s --start-period=20s --retries=3 \
  CMD curl --fail http://localhost:6969/api/v1/hello || exit 1
