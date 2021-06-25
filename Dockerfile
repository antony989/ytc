FROM golang:1.16-alpine AS builder
WORKDIR /go/src/app
COPY . .
RUN go mod download
RUN go build -o main .
CMD ["./main"]