FROM golang:1.23-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -o /chat-server ./cmd/chat-server

FROM alpine:3.21
RUN apk add --no-cache ca-certificates
COPY --from=build /chat-server /usr/local/bin/chat-server
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/chat-server"]
