FROM alpine:3.23 AS pigz-builder

RUN apk add --no-cache build-base zlib-dev zlib-static
WORKDIR /src
RUN wget -q https://zlib.net/pigz/pigz-2.8.tar.gz \
    && echo "eb872b4f0e1f0ebe59c9f7bd8c506c4204893ba6a8492de31df416f0d5170fd0  pigz-2.8.tar.gz" | sha256sum -c - \
    && tar -xzf pigz-2.8.tar.gz \
    && make -C pigz-2.8 LDFLAGS=-static \
    && strip pigz-2.8/pigz

FROM golang:1.27.1-alpine3.23 AS builder

ARG TARGETOS
ARG TARGETARCH
ENV CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH}

WORKDIR /app

COPY go.mod go.sum /app/
RUN go mod download

COPY . /app
RUN go build -o mariadb-operator cmd/controller/*.go

FROM gcr.io/distroless/static AS app

WORKDIR /
COPY --from=builder /app/mariadb-operator /bin/mariadb-operator 
COPY --from=pigz-builder /src/pigz-2.8/pigz /bin/pigz
COPY --from=pigz-builder /src/pigz-2.8/README /licenses/pigz.txt
COPY --from=pigz-builder /src/pigz-2.8/zopfli/COPYING /licenses/zopfli.txt
USER 65532:65532

ENTRYPOINT ["/bin/mariadb-operator"]
