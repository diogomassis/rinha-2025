FROM golang:1.23-alpine

RUN apk add --no-cache make protobuf-dev bash

RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@latest && \
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest && \
    go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway@latest

WORKDIR /app

COPY . .

RUN make all

EXPOSE 3030 8080

CMD ["./bin/server"]
