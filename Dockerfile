FROM golang:1.27.3-alpine3.23@sha256:8a5046497cda39860b832ffdbe8dd66018a0f05bb01352f533a36e7e2ced66a0 AS builder
RUN apk add --no-cache make bash git
WORKDIR /src/
COPY . /src/
RUN make install
