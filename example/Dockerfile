FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /api ./cmd/api
RUN CGO_ENABLED=0 go build -trimpath -o /grpc-backend ./cmd/grpc-backend
RUN CGO_ENABLED=0 go build -trimpath -o /connect-backend ./cmd/connect-backend
RUN CGO_ENABLED=0 go build -trimpath -o /http-backend ./cmd/http-backend
RUN CGO_ENABLED=0 go build -trimpath -o /client ./cmd/client

FROM alpine:3.21 AS api
RUN addgroup -S app && adduser -S -G app app
COPY --from=builder /api /usr/local/bin/api
USER app
ENTRYPOINT ["api"]

FROM alpine:3.21 AS grpc-backend
RUN addgroup -S app && adduser -S -G app app
COPY --from=builder /grpc-backend /usr/local/bin/grpc-backend
USER app
ENTRYPOINT ["grpc-backend"]

FROM alpine:3.21 AS connect-backend
RUN addgroup -S app && adduser -S -G app app
COPY --from=builder /connect-backend /usr/local/bin/connect-backend
USER app
ENTRYPOINT ["connect-backend"]

FROM alpine:3.21 AS http-backend
RUN addgroup -S app && adduser -S -G app app
COPY --from=builder /http-backend /usr/local/bin/http-backend
USER app
ENTRYPOINT ["http-backend"]

FROM alpine:3.21 AS client
RUN addgroup -S app && adduser -S -G app app
COPY --from=builder /client /usr/local/bin/client
USER app
ENTRYPOINT ["client"]
