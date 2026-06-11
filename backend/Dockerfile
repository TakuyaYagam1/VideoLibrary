# syntax=docker/dockerfile:1.7

ARG GO_VERSION=1.26.4

FROM golang:${GO_VERSION}-bookworm AS build

WORKDIR /src

ENV CGO_ENABLED=0 \
    GOOS=linux \
    GOFLAGS=-mod=readonly

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -trimpath -ldflags="-s -w" -o /out/videolibrary ./cmd/app

FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app

COPY --from=build /out/videolibrary /app/videolibrary
COPY --from=build /src/migrations /app/migrations

USER nonroot:nonroot
EXPOSE 8080

ENTRYPOINT ["/app/videolibrary"]
