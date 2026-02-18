FROM golang:1.21 AS builder
WORKDIR /app
COPY . .
RUN go build -o alsamixer-web ./cmd/alsamixer-web

FROM alpine:latest
RUN adduser -D -u 1000 alsamixer
COPY --from=builder /app/alsamixer-web /usr/local/bin/
USER alsamixer
EXPOSE 8080
ENTRYPOINT ["alsamixer-web"]