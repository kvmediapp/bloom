# Build from the server/ directory:  docker build -t bloom-server server/
FROM golang:1.24 AS build
WORKDIR /src
COPY go.mod ./
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /out/bloomd ./cmd/bloomd

FROM debian:bookworm-slim
RUN apt-get update \
 && apt-get install -y --no-install-recommends ffmpeg ca-certificates \
 && rm -rf /var/lib/apt/lists/*
COPY --from=build /out/bloomd /usr/local/bin/bloomd
ENV BLOOM_MEDIA_DIR=/data/media \
    BLOOM_CACHE_DIR=/config \
    BLOOM_ADDR=:8484
VOLUME /config
EXPOSE 8484
ENTRYPOINT ["bloomd"]
