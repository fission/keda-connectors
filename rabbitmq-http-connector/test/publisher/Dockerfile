FROM golang:1.18-alpine as build

RUN apk add --no-cache git

RUN mkdir /src
WORKDIR /src
COPY go.mod . 
COPY go.sum .

RUN go mod download

# Copy source code to image
COPY . .

RUN go build publish-msgs.go

##FROM alpine as runtime

##COPY --from=build /src/publisher /app/publisher

CMD [ "./publish-msgs" ]
