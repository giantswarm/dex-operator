FROM alpine:3.20.3

RUN apk add --no-cache ca-certificates

ADD ./dex-operator /dex-operator

ENTRYPOINT ["/dex-operator"]
