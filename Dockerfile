FROM alpine:latest
RUN apk --update upgrade && apk add curl ca-certificates && \
    update-ca-certificates && rm -rf /var/cache/apk/*
RUN apk add --no-cache musl-dev go git tree

# Dependencies
RUN go get github.com/mediocregopher/markov/markovbot/slack github.com/mediocregopher/lever github.com/mediocregopher/radix.v2

# Copy files, modify this with source changes
ADD . /app
WORKDIR /app

# Environment
ENV MARKOV_REDISADDR redis:6379

# Build binary
RUN go build -o markov
EXPOSE 8080
CMD ["./markov"]