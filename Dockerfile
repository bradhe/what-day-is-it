FROM golang:1.13-alpine AS builder

ENV GO111MODULE on
ENV CGO_ENABLED 0

WORKDIR /go/src/github.com/bradhe/what-day-is-it
COPY . .
RUN mkdir -p ./bin
RUN go build -a -installsuffix cgo -mod vendor -o ./bin/what-day-is-it ./cmd/what-day-is-it

FROM alpine:latest
RUN apk update
RUN apk --no-cache add git gcc bind-dev musl-dev ca-certificates
RUN update-ca-certificates
COPY --from=builder /go/src/github.com/bradhe/what-day-is-it/bin/what-day-is-it /usr/bin/what-day-is-it
CMD ["what-day-is-it"]