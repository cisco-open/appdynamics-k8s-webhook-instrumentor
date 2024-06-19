############################
# STEP 1 build executable binary
############################
FROM golang:1.22.1-alpine3.19 AS builder
# Install git.
# Git is required for fetching the dependencies.
RUN apk update && apk add --no-cache git
WORKDIR $GOPATH/src/webhook/
COPY pkg .
# Fetch dependencies.
RUN go mod tidy
# Build the binary.
RUN GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o /go/bin/webhook

############################
# STEP 3 build a small image
############################
FROM alpine:3.19
# Copy our static executable.
COPY --from=builder /go/bin/webhook /go/bin/webhook

EXPOSE 8443

CMD ["/go/bin/webhook"]