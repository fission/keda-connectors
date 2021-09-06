FROM golang:1.15-alpine as build

RUN apk add --no-cache git

WORKDIR /src 

RUN go get github.com/sirupsen/logrus
RUN go get github.com/streadway/amqp

COPY consumer-msgs.go /src 

RUN go build consumer-msgs.go

CMD [ "./consumer-msgs" ]
