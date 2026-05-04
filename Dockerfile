# syntax=docker/dockerfile:1.7

FROM golang:1.22-alpine AS build

WORKDIR /src

ARG GOPROXY=https://proxy.golang.org,direct
ENV GOPROXY=$GOPROXY

RUN apk add --no-cache git

COPY go.mod go.sum* ./
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go get modernc.org/sqlite@v1.36.0 && go mod download

COPY cmd ./cmd
COPY internal ./internal

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -o /out/vibecode-helper ./cmd/vibecode-helper

FROM alpine:3.20

COPY --from=build /out/vibecode-helper /usr/local/bin/vibecode-helper

EXPOSE 8766

ENTRYPOINT ["vibecode-helper"]
CMD ["serve", "--addr", "0.0.0.0:8766"]
