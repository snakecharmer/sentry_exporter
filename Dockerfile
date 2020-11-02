FROM golang:1.15.3 AS builder

WORKDIR /src/go/github.com/snakecharmer/sentry_exporter

COPY . .

RUN go get -d -v
RUN go build -o sentry_exporter .

FROM alpine:latest  
RUN apk --no-cache add ca-certificates

WORKDIR /app/

# https://stackoverflow.com/a/35613430
RUN mkdir -p /etc/sentry_exporter/ && mkdir /lib64 && \
    ln -s /lib/libc.musl-x86_64.so.1 /lib64/ld-linux-x86-64.so.2

COPY --from=builder /src/go/github.com/snakecharmer/sentry_exporter .
COPY sentry_exporter.yml /etc/sentry_exporter/config.yml

EXPOSE 9412
ENTRYPOINT ["./sentry_exporter"]
CMD ["--config.file=/etc/sentry_exporter/config.yml"]