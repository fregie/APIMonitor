FROM golang:alpine AS build-env
RUN apk --no-cache add ca-certificates

FROM debian:stable-slim
COPY --from=build-env /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY APITest /APITest
COPY APITest.yaml /APITest.yaml

ENTRYPOINT ["bash", "-c", "/APITest -c /APITest.yaml"]