FROM alpine:latest
RUN apk --update upgrade && apk add curl ca-certificates && \
    update-ca-certificates && rm -rf /var/cache/apk/*
RUN apk add --no-cache musl-dev go git tree

# Dependencies
RUN go get github.com/mediocregopher/markov/markovbot/slack
RUN go get github.com/mediocregopher/lever
RUN go get github.com/mediocregopher/radix.v2

# Copy files, modify this with source changes
WORKDIR /app
COPY main.go /app/
COPY builder/ /app/builder/
COPY markovbot/ /app/markovbot/

# CLI Arguments
ENV REDIS_ADDR='redis:6379'
ENV LISTEN_ADDR=':8080'
ENV PREFIX_LEN='2'
ENV TIMEOUT='720'

# Build binary
RUN go build -o markov
RUN chmod +x markov
EXPOSE 8080
CMD ["sh", "-c", "./markov -redisAddr $REDIS_ADDR -timeout $TIMEOUT -prefixLen $PREFIX_LEN -listenAddr=$LISTEN_ADDR"]