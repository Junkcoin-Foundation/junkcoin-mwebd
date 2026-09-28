# This Dockerfile builds ltcd from source and creates a small (55 MB) docker container based on alpine linux.
#
# Clone this repository and run the following command to build and tag a fresh ltcd amd64 container:
#
# docker build . -t yourregistry/ltcd
#
# You can use the following command to buid an arm64v8 container:
#
# docker build . -t yourregistry/ltcd --build-arg ARCH=arm64v8
#
# For more information how to use this docker image visit:
# https://github.com/ltcsuite/ltcd/tree/master/docs
#
# 9333  Mainnet Litecoin peer-to-peer port
# 9334  Mainet RPC port

ARG ARCH=amd64
# using the SHA256 instead of tags
# https://github.com/opencontainers/image-spec/blob/main/descriptor.md#digests
# https://cloud.google.com/architecture/using-container-images
# https://github.com/google/go-containerregistry/blob/main/cmd/crane/README.md
# ➜  ~ crane digest golang:1.17.13-alpine3.16
# sha256:c80567372be0d486766593cc722d3401038e2f150a0f6c5c719caa63afb4026a
FROM golang@sha256:c80567372be0d486766593cc722d3401038e2f150a0f6c5c719caa63afb4026a AS build-container

ARG ARCH
ENV GO111MODULE=on

ADD . /app
WORKDIR /app
RUN set -ex \
  && if [ "${ARCH}" = "amd64" ]; then export GOARCH=amd64; fi \
  && if [ "${ARCH}" = "arm32v7" ]; then export GOARCH=arm; fi \
  && if [ "${ARCH}" = "arm64v8" ]; then export GOARCH=arm64; fi \
  && echo "Compiling for $GOARCH" \
  && go install -v . ./cmd/...

FROM $ARCH/alpine:3.16

COPY --from=build-container /go/bin /bin

VOLUME ["/root/.ltcd"]

EXPOSE 9333 9334

ENTRYPOINT ["ltcd"]
