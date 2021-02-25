FROM golang:1.16.0-alpine3.12 AS builder
RUN apk add --no-cache make bash git
WORKDIR /src/
COPY . /src/
RUN make install
