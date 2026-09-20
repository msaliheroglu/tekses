# Yük üreteci imajı — kök bağlamla derlenir:
#   docker build -f tools/loadgen/Dockerfile -t tekses-loadgen .
#
# VM'de gateway'e compose ağı üzerinden, Caddy/TLS'e uğramadan bağlanır
# (ölçülen şey gateway'in kendisidir). Dosya tanıtıcısı ve yerel port
# aralığı konteyner başına ayarlanır; kullanım docs/yuk-testi.md'de.
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY packages/ packages/
COPY tools/loadgen/ tools/loadgen/
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/loadgen ./tools/loadgen

FROM alpine:3.20
COPY --from=build /out/loadgen /usr/local/bin/loadgen
ENTRYPOINT ["loadgen"]
