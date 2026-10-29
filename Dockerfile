# ===========
# Build stage
# ===========
FROM golang:1.27.3-alpine AS builder

WORKDIR /code

ENV CGO_ENABLED=0

ARG VERSION=unknown
ARG REVISION=unknown
ARG CREATED=unknown

# Pre-install dependencies to cache them as a separate image layer
COPY go.mod go.sum ./
RUN go mod download

# Build
COPY . /code
RUN go build -trimpath -ldflags="-s -w -X github.com/prometheus/common/version.Version=${VERSION} -X github.com/prometheus/common/version.Revision=${REVISION} -X github.com/prometheus/common/version.BuildDate=${CREATED}" -o sonic-exporter ./cmd/sonic-exporter

# ===========
# Final stage
# ===========
FROM alpine:3.24

ARG VERSION=unknown
ARG REVISION=unknown
ARG CREATED=unknown
ARG SOURCE=https://github.com/premday/sonic-exporter

LABEL org.opencontainers.image.title="sonic-exporter" \
	org.opencontainers.image.description="Prometheus exporter for SONiC switches" \
	org.opencontainers.image.version="${VERSION}" \
	org.opencontainers.image.revision="${REVISION}" \
	org.opencontainers.image.created="${CREATED}" \
	org.opencontainers.image.source="${SOURCE}"

ENV SONIC_EXPORTER_WEB_VRF=mgmt \
	SONIC_EXPORTER_LISTEN_ADDRESS=:9101 \
	SONIC_EXPORTER_ROOTFS=/hostfs

WORKDIR /app
COPY --from=builder /code/sonic-exporter ./sonic-exporter
RUN apk --no-cache add bash supervisor libcap \
	&& setcap cap_net_raw=ep /app/sonic-exporter \
	&& apk del libcap \
	&& addgroup -S sonic \
	&& adduser -S -G sonic sonic \
	&& mkdir -p /run/sonic-supervisor \
	&& chown sonic:sonic /run/sonic-supervisor \
	&& chmod 0700 /run/sonic-supervisor
COPY supervisord.conf /etc/supervisord.conf

EXPOSE 9101

USER sonic

CMD [ "/usr/bin/supervisord", "-c", "/etc/supervisord.conf" ]
