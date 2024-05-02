FROM --platform=${BUILDPLATFORM:-linux/amd64} golang:1.22 as builder

ARG TARGETPLATFORM
ARG BUILDPLATFORM
ARG TARGETOS
ARG TARGETARCH

WORKDIR /app/
ADD . .
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -ldflags="-w -s" -o /hems-exporter .

FROM --platform=${TARGETPLATFORM:-linux/amd64} gcr.io/distroless/static-debian12
COPY --from=builder /hems-exporter /hems-exporter

ENTRYPOINT ["/hems-exporter"]
