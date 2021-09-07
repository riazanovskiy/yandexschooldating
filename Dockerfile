FROM golang:1.17 AS build-env-base
RUN go get github.com/go-delve/delve/cmd/dlv
WORKDIR /dockerdev
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
ADD . /dockerdev

FROM build-env-base as build-env
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build CGO_ENABLED=0 go build -o /server

FROM gcr.io/distroless/static-debian10 as yandexdating
COPY --from=build-env /server /
ENTRYPOINT ["/server"]
CMD ["/run/secrets/bot_token"]

FROM build-env-base AS build-env-debug
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build CGO_ENABLED=0 go build -gcflags="all=-N -l" -o /server

FROM gcr.io/distroless/base-debian10:debug as yandexdating-debug
COPY --from=build-env-debug /go/bin/dlv /
COPY --from=build-env-debug /server /
ENTRYPOINT ["/dlv"]
CMD ["--listen=:40000", "--headless=true", "--api-version=2", "--accept-multiclient", "exec", "/server", "/run/secrets/bot_token"]
