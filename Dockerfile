# Build
FROM golang:1.13-alpine3.10 AS build
RUN apk add --no-cache make git protobuf protobuf-dev curl && \
    rm -rf /var/cache/apk/*
ENV CGO_ENABLED 0
ENV GOOS linux
ENV GOPRIVATE git.coinninja.net
WORKDIR /build
COPY . .
RUN make

# Production
FROM alpine:3.10
RUN apk add --no-cache ca-certificates su-exec && \
    rm -rf /var/cache/apk/*
RUN addgroup -S blocc && adduser -S blocc -G blocc
RUN mkdir -p /opt/blocc
WORKDIR /opt/blocc
EXPOSE 8080
COPY --from=build /build/bloccapi .
CMD [ "su-exec", "blocc:blocc", "/opt/blocc/bloccapi" ]
