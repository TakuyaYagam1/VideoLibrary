FROM golang:1.26.4-alpine3.24 AS builder

WORKDIR /src

ENV CGO_ENABLED=0 \
    GOOS=linux \
    GOFLAGS=-mod=readonly

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -trimpath -ldflags="-s -w" -o /out/videolibrary ./cmd/app

FROM alpine:3.24.0

WORKDIR /app

RUN addgroup -S -g 10001 videolibrary \
    && adduser -S -D -H -u 10001 -G videolibrary videolibrary

COPY --from=builder /out/videolibrary /app/videolibrary
COPY --from=builder /src/migrations /app/migrations

USER videolibrary:videolibrary
EXPOSE 8080

ENTRYPOINT ["/app/videolibrary"]
