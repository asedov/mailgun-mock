FROM golang:alpine AS builder

WORKDIR /app

COPY go.mod .
COPY go.sum .
COPY mailgun-mock.go .

# @see https://medium.com/@diogok/on-golang-static-binaries-cross-compiling-and-plugins-1aed33499671
RUN apk add --no-cache git \
 && go build -a -tags netgo -ldflags '-w -extldflags "-static"' -o mailgun-mock mailgun-mock.go


FROM scratch
COPY --from=builder /app/mailgun-mock .
COPY public /public
ENTRYPOINT ["/mailgun-mock"]
