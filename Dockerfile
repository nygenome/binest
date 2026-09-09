FROM golang:1.27.1-alpine3.23@sha256:d9e2f2f07b10cc922da3e80e035c3058810b328d5aef82d2c63680967c5e2ec9 AS builder
RUN apk add --no-cache make bash git
WORKDIR /src/
COPY . /src/
RUN make install
