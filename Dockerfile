FROM alpine:3.24.2

RUN apk add --no-cache ca-certificates

ARG TARGETARCH
COPY dex-operator-linux-${TARGETARCH} /dex-operator

ENTRYPOINT ["/dex-operator"]
