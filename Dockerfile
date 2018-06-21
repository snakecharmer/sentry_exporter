FROM quay.io/prometheus/busybox:latest

COPY sentry_exporter_linux_amd64  /bin/sentry_exporter
COPY sentry.yml       /etc/sentry_exporter/config.yml

EXPOSE      9412
ENTRYPOINT  [ "/bin/sentry_exporter" ]
CMD         [ "-config.file=/etc/sentry_exporter/config.yml" ]
