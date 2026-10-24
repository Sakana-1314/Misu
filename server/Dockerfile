FROM golang:1.27-alpine AS build

ARG GOPROXY=https://proxy.golang.org,direct
ENV GOPROXY=${GOPROXY}

WORKDIR /src
RUN apk add --no-cache git ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG VERSION=dev
ARG COMMIT=unknown

RUN go test ./... && \
    CGO_ENABLED=0 GOOS=linux go build -trimpath \
      -ldflags="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT}" \
      -o /out/misu-server ./cmd/server

FROM alpine:3.20

RUN apk add --no-cache ca-certificates && \
    addgroup -S misu && adduser -S -G misu misu

COPY --from=build /out/misu-server /usr/local/bin/misu-server

ENV TZ=Asia/Shanghai \
    SERVER_PORT=8080

USER misu
EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=20s --retries=5 \
  CMD wget -q -O - http://127.0.0.1:8080/api/health || exit 1

ENTRYPOINT ["/usr/local/bin/misu-server"]
