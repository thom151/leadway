FROM --platform=linux/amd64 debian:stable-slim

RUN apt-get update && apt-get install -y ca-certificates

ADD templates ./templates
ADD static ./static
ADD assets ./assets
ADD leadme /usr/bin/leadme

RUN apt-get update && apt-get install -y \
    ffmpeg \
    && rm -rf /var/lib/apt/lists/*

CMD ["leadme"]
