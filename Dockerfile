FROM alpine:3.19.0

RUN apk add --no-cache ca-certificates

ADD ./dex-operator /dex-operator

ENTRYPOINT ["/dex-operator"]
