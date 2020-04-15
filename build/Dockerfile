FROM alpine as builder
RUN apk update && apk add ca-certificates


FROM busybox:glibc
COPY --from=builder /etc/ssl/certs /etc/ssl/certs
COPY bin/elasticsearch-plugin /usr/local/bin/elasticsearch-plugin
RUN chmod +x /usr/local/bin/elasticsearch-plugin

ENTRYPOINT ["elasticsearch-plugin"]
