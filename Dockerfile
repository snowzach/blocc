# Build
FROM golang:1.11-alpine3.8 AS build
RUN apk add --no-cache make git && \
    rm -rf /var/cache/apk/*
ENV CGO_ENABLED 0
ENV GOOS linux
WORKDIR /build
COPY . .
RUN make

# Production
FROM alpine:3.8
RUN apk add --no-cache ca-certificates && \
    rm -rf /var/cache/apk/*
RUN addgroup -S blocc && adduser -S blocc -G blocc
RUN mkdir -p /opt/blocc
USER blocc
WORKDIR /opt/blocc
COPY --from=build /build/bloccapi .
EXPOSE 8080
CMD [ "/opt/blocc/bloccapi", "btc" ]
