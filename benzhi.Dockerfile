# syntax=docker/dockerfile:1.7
FROM golang:1.23-bookworm
WORKDIR /workspace
COPY go.mod ./
RUN go mod download
COPY . .
RUN go build ./...
ENV GOPROXY=off GOSUMDB=off
CMD ["bash"]
