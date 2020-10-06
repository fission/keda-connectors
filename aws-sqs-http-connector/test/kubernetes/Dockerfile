
FROM localhost:5000/aws-sqs-connector:latest

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