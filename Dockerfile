FROM alpine:3.23.4

RUN apk add --no-cache ca-certificates

ADD ./dex-operator /dex-operator

ENTRYPOINT ["/dex-operator"]
