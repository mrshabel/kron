# support for c-bindings for kafka client: https://stackoverflow.com/questions/76177554/how-can-i-fix-my-docker-build-failing-while-my-go-build-succeeds-dockerfile-inc

# builder stage
FROM golang:1.24-alpine AS builder

# install necessary deps
RUN apk add --no-progress --no-cache gcc musl-dev

WORKDIR /app

# install deps
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# build all binaries (producer and consumer)
RUN go build -tags musl -ldflags '-extldflags "-static"' -o producer cmd/producer/main.go
RUN go build -tags musl -ldflags '-extldflags "-static"' -o consumer cmd/consumer/main.go


# producer image
FROM alpine AS producer

WORKDIR /app

# copy binary from builder stage
COPY --from=builder /app/producer /app/main
COPY --from=builder /app/tabs /app/tabs

CMD ["./main"]


# consumer image
FROM alpine AS consumer

WORKDIR /app

# copy binary from builder stage
COPY --from=builder /app/consumer /app/main

CMD ["./main"]
