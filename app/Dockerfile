FROM golang:1.22-alpine

WORKDIR /app

# Iniciar o go mod e baixar dependências antes de copiar o resto
# para aproveitar o cache do Docker.
COPY go.mod ./
RUN go mod download
RUN go mod tidy

COPY . .

RUN go build -o /server .

EXPOSE 8080

CMD [ "/server" ]
