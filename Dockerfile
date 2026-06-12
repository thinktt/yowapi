FROM alpine

COPY dist/yowapi /opt
COPY dist/personalities.json /opt
COPY dist/certs /opt/certs
WORKDIR /opt

ENV GIN_MODE=release
CMD ["./yowapi"]
