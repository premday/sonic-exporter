# ===========
# Build stage
# ===========
FROM golang:1.26-alpine AS builder

WORKDIR /code

ENV CGO_ENABLED=0

# Pre-install dependencies to cache them as a separate image layer
COPY go.mod go.sum ./
RUN go mod download

# Build
COPY . /code
RUN go build -trimpath -ldflags="-s -w" -o sonic-exporter ./cmd/sonic-exporter

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

WORKDIR /app
COPY --from=builder /code/sonic-exporter ./sonic-exporter
RUN apk --no-cache add libcap \
	&& setcap cap_net_raw=ep /app/sonic-exporter \
	&& apk del libcap \
	&& addgroup -S sonic \
	&& adduser -S -G sonic sonic

EXPOSE 9101

USER sonic

CMD [ "./sonic-exporter" ]
