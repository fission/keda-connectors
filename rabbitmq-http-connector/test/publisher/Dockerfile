FROM golang:1.15-alpine as build

RUN apk add --no-cache git

WORKDIR /src 

RUN go get github.com/julienschmidt/httprouter
RUN go get github.com/sirupsen/logrus
RUN go get github.com/streadway/amqp
COPY publish-msgs.go /src 

RUN go build publish-msgs.go

##FROM alpine as runtime

##COPY --from=build /src/publisher /app/publisher

CMD [ "./publish-msgs" ]
