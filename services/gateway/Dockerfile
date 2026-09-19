# Kök bağlamla derlenir: docker build -f services/gateway/Dockerfile .
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY packages/ packages/
COPY services/gateway/ services/gateway/
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/gateway ./services/gateway/cmd/gateway

FROM alpine:3.20
RUN adduser -D -H tekses
USER tekses
COPY --from=build /out/gateway /usr/local/bin/gateway
EXPOSE 8080
ENTRYPOINT ["gateway"]
