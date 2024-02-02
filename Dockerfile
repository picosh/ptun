FROM golang:1.21-alpine

WORKDIR /app

COPY go.* .

RUN go mod download

COPY . .

RUN go build -o docker cmd/docker/main.go

CMD [ "/app/docker" ]