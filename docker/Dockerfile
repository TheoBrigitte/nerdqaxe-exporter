# Image built by GoReleaser, using the binaries it already compiled.
# The build context only contains the binaries, laid out as <os>/<arch>/<binary>.
FROM gcr.io/distroless/static-debian13:latest
ARG TARGETPLATFORM

COPY $TARGETPLATFORM/bin/nerdqaxe-exporter /app/nerdqaxe-exporter

EXPOSE 10055

ENTRYPOINT ["/app/nerdqaxe-exporter"]
