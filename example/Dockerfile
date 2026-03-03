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
COPY --from=builder /api /usr/local/bin/api
ENTRYPOINT ["api"]

FROM alpine:3.21 AS grpc-backend
COPY --from=builder /grpc-backend /usr/local/bin/grpc-backend
ENTRYPOINT ["grpc-backend"]

FROM alpine:3.21 AS connect-backend
COPY --from=builder /connect-backend /usr/local/bin/connect-backend
ENTRYPOINT ["connect-backend"]

FROM alpine:3.21 AS http-backend
COPY --from=builder /http-backend /usr/local/bin/http-backend
ENTRYPOINT ["http-backend"]

FROM alpine:3.21 AS client
COPY --from=builder /client /usr/local/bin/client
ENTRYPOINT ["client"]
