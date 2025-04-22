FROM golang:bullseye AS builder
RUN apt-get update && apt-get install -y upx
ADD . /luet
RUN cd /luet && make build

FROM scratch
ENV LUET_NOLOCK=true
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /luet/luet /usr/bin/luet

ENTRYPOINT ["/usr/bin/luet"]
