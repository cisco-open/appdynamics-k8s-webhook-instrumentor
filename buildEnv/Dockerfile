############################
# STEP 1 build executable binary
############################
FROM golang:alpine AS builder
# Install git.
# Git is required for fetching the dependencies.
RUN apk update && apk add --no-cache git
WORKDIR $GOPATH/src/gargoyle/webhook/
COPY webhook .
# Fetch dependencies.
# Using go get.
RUN go get -d -v
# Build the binary.
RUN GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o /go/bin/webhook

############################
# STEP 3 build a small image
############################
FROM alpine:latest
# Copy our static executable.
COPY --from=builder /go/bin/webhook /go/bin/webhook

EXPOSE 8443

CMD ["/go/bin/webhook"]