FROM golang:1.17.6 as builder

WORKDIR /tmp/taxes

COPY . .

ARG BUILDER
ARG VERSION

ENV TAXES_BUILDER=${BUILDER}
ENV TAXES_VERSION=${VERSION}

RUN apt-get update && apt-get install make git gcc -y && \
    make build_deps && \
    make

FROM alpine:latest

WORKDIR /app

COPY --from=builder /tmp/taxes/bin/taxes .

CMD ["/app/taxes"]
