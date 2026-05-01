FROM golang:1.22-alpine AS build

WORKDIR /src

COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal

RUN go build -o /out/vibecode-helper ./cmd/vibecode-helper

FROM alpine:3.20

COPY --from=build /out/vibecode-helper /usr/local/bin/vibecode-helper

EXPOSE 8766

ENTRYPOINT ["vibecode-helper"]
CMD ["serve", "--addr", "0.0.0.0:8766"]
