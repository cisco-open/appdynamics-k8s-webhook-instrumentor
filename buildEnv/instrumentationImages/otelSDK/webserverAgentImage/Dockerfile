############################
# STEP 1 build executable binary
############################
FROM golang:alpine AS builder
# Install git.
# Git is required for fetching the dependencies.
RUN apk update && apk add --no-cache git
WORKDIR $GOPATH/src/helpers/patcher/
COPY helpers/nginxConfPatcher .
# Fetch dependencies.
# Using go get.
RUN go get -d -v
# Build the binary.
RUN GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o /go/bin/nginxConfPatcher


############################
# STEP 2 build agent image
############################

FROM alpine:latest

COPY ./build/agent/opentelemetry-webserver-sdk /opt/opentelemetry
COPY --from=builder /go/bin/nginxConfPatcher /opt/opentelemetry

RUN chmod a+w /opt/opentelemetry/logs

CMD ["cat", "Just delivering the Opentelemetry Apache/Nginx agent"]
