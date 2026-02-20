FROM golang:1.23-bookworm

RUN apt-get update \
    && apt-get install -y --no-install-recommends libasound2-dev \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /src
