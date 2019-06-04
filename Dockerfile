FROM  quay.io/prometheus/busybox:latest
LABEL maintainer="Lukas Malkmus <mail@lukasmalkmus.com>"

COPY rpi_exporter /bin/rpi_exporter

ENTRYPOINT ["/bin/rpi_exporter"]
EXPOSE     9243
