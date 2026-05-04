FROM golang:1.22-alpine AS build

WORKDIR /src

ARG GOPROXY=https://proxy.golang.org,direct
ENV GOPROXY=$GOPROXY

RUN apk add --no-cache git

COPY go.mod go.sum* ./
COPY cmd ./cmd
COPY internal ./internal

RUN go mod tidy && go build -o /out/vibecode-helper ./cmd/vibecode-helper

FROM alpine:3.20

COPY --from=build /out/vibecode-helper /usr/local/bin/vibecode-helper

EXPOSE 8766

ENTRYPOINT ["vibecode-helper"]
CMD ["serve", "--addr", "0.0.0.0:8766"]
