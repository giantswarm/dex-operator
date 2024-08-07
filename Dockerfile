FROM alpine:3.20.2

RUN apk add --no-cache ca-certificates

ADD ./dex-operator /dex-operator

ENTRYPOINT ["/dex-operator"]
