FROM golang:1.16 as build

ARG LD_FLAGS

WORKDIR /go/src/github.com/ztroop/ontario-aqhi-exporter
COPY . .

RUN go version
RUN go mod vendor
RUN GO111MODULE=on CGO_ENABLED=1 go test -race -mod vendor ./...
RUN GO111MODULE=on CGO_ENABLED=0 go build -mod vendor -ldflags "$LD_FLAGS" -o /go/bin/app .

FROM alpine:3.14

COPY --from=build /go/bin/app /
ENTRYPOINT ["/app"]