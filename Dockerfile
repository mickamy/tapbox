FROM golang:1.26-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /tapbox .

FROM alpine:3.21
RUN addgroup -S tapbox && adduser -S -G tapbox tapbox
COPY --from=builder /tapbox /usr/local/bin/tapbox
USER tapbox
ENTRYPOINT ["tapbox"]
