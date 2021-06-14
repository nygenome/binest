FROM golang:1.16.5-alpine3.13 AS builder
RUN apk add --no-cache make bash git
WORKDIR /src/
COPY . /src/
RUN make install
