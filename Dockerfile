FROM golang as builder
RUN apt-get update && apt-get install upx
ADD . /luet
RUN cd /luet && make build-small

FROM scratch
ENV LUET_NOLOCK=true
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /luet/luet /usr/bin/luet

ENTRYPOINT ["/usr/bin/luet"]
