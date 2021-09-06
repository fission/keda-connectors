
FROM golang:1.15-alpine as builder

RUN apk add bash ca-certificates git gcc g++ libc-dev


RUN mkdir /app
WORKDIR /app
COPY go.mod . 
COPY go.sum .

RUN go mod download

# Copy source code to image
COPY . .

RUN go build -a -o /go/bin/main
FROM alpine:3.14 as base
RUN apk add --update ca-certificates
COPY --from=builder /go/bin/main /
RUN apk add --no-cache \
        python3 \
        py3-pip \
    && pip3 install --upgrade pip \
    && pip3 install \
        awscli \
    && rm -rf /var/cache/apk/*
COPY ./aws_setup.sh .
RUN chmod +x ./aws_setup.sh 
RUN sh ./aws_setup.sh  

ENTRYPOINT ["/main"]