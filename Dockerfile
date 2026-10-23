FROM alpine:3.24.3

RUN apk add --no-cache ca-certificates

ARG TARGETARCH
COPY dex-operator-linux-${TARGETARCH} /dex-operator

ENTRYPOINT ["/dex-operator"]
