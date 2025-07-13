# syntax=docker/dockerfile:1

FROM golang:1.21-alpine AS builder

RUN apk add --no-cache git

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-extldflags "-static"' -o photo-sorter ./cmd/photo-sorter

FROM alpine:latest

RUN apk --no-cache add ca-certificates ffmpeg

RUN addgroup -g 1001 -S photosorter && \
    adduser -u 1001 -S photosorter -G photosorter

WORKDIR /app

COPY --from=builder /app/photo-sorter .

COPY --from=builder /app/config.example.yaml ./config.example.yaml

RUN mkdir -p /photos /logs && \
    chown -R photosorter:photosorter /app /photos /logs

USER photosorter

VOLUME ["/photos", "/logs"]

ENV PHOTO_SORTER_LOGGING_FILE_PATH=/logs/photo-sorter.log
ENV PHOTO_SORTER_SOURCE_DIRECTORY=/photos

ENTRYPOINT ["./photo-sorter"]
CMD ["--help"]
