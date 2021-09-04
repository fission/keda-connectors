FROM golang:1.15-alpine
RUN mkdir /server
WORKDIR /server
COPY . .
EXPOSE 8080
ENTRYPOINT ["go", "run", "main.go"]
