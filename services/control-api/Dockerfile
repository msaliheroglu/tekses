# Kök bağlamla derlenir: docker build -f services/control-api/Dockerfile .
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY packages/ packages/
COPY services/control-api/ services/control-api/
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/control-api ./services/control-api/cmd/control-api

FROM alpine:3.20
# UID 1000 sabittir: packages disk bölümü whisper varyantıyla
# (Dockerfile.whisper) paylaşılır; iki imajın kullanıcısı aynı UID'de olmalı.
RUN adduser -D -H -u 1000 tekses && mkdir -p /data/packages && chown tekses /data/packages
USER tekses
COPY --from=build /out/control-api /usr/local/bin/control-api
ENV TEKSES_PACKAGES_DIR=/data/packages
VOLUME /data/packages
EXPOSE 8090
ENTRYPOINT ["control-api"]
