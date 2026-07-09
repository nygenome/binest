FROM golang:1.26.5-alpine3.23@sha256:622e56dbc11a8cfe87cafa2331e9a201877271cbff918af53d3be315f3da88cc AS builder
RUN apk add --no-cache make bash git
WORKDIR /src/
COPY . /src/
RUN make install
