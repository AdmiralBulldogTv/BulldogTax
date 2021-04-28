FROM golang:1.16.3-alpine3.13 AS build_base

RUN apk add --no-cache git

# Set the Current Working Directory inside the container
WORKDIR /tmp/app

# We want to populate the module cache based on the go.{mod,sum} files.
COPY go.mod .
COPY go.sum .

RUN go mod download

COPY . .

# Build the Go app
RUN go build

# Start fresh from a smaller image
FROM alpine:3.9 
RUN apk add ca-certificates

WORKDIR /app

COPY --from=build_base /tmp/app/bulldog-taxes /app/bulldog-taxes
COPY config.yaml /app/config.yaml

# This container exposes port 8080 to the outside world
EXPOSE 8080

# Run the binary program produced by `go install`
CMD ["/app/bulldog-taxes"]
