FROM golang:1.26-alpine AS builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN set -ex \
    && apk add --no-cache --virtual .build-deps \
    gcc libc-dev \
    && go build -ldflags '-extldflags "-static"' -o docker-volume-sshfs . \
    && apk del .build-deps
CMD ["/build/docker-volume-sshfs"]

FROM alpine
RUN apk update && apk add sshfs
RUN mkdir -p /run/docker/plugins /mnt/state /mnt/volumes
COPY --from=builder /build/docker-volume-sshfs .
CMD ["docker-volume-sshfs"]
