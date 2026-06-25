FROM golang:1.26.4-alpine3.23@sha256:18b460dd17542c2ba43299a633cf6ebfc1115101509531471d7cfce1019af083 AS builder
RUN apk add --no-cache make bash git
WORKDIR /src/
COPY . /src/
RUN make install
