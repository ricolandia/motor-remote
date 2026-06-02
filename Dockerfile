FROM golang:1.25-alpine AS builder
RUN apk add --no-cache gcc musl-dev sqlite-dev
WORKDIR /app
COPY server/ .
RUN go build -o /remote-server ./cmd/server

FROM alpine:3.21
RUN apk add --no-cache sqlite ca-certificates tzdata
COPY --from=builder /remote-server /
COPY web/ /web/
COPY db/schema.sql /db/schema.sql
COPY docker-entrypoint.sh /
RUN chmod +x /docker-entrypoint.sh
EXPOSE 8080 2222
ENV DB_PATH=/data/game.db
ENV STATIC_DIR=/web/static
ENTRYPOINT ["/docker-entrypoint.sh"]
