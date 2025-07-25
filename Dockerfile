FROM golang:1.23-alpine

RUN apk add --no-cache make protobuf-dev bash

WORKDIR /app

COPY . .

RUN make install-tools
RUN make all

EXPOSE 3030 8080

CMD ["./bin/server"]
