FROM golang:1.10

WORKDIR /go/src/github.com/gazure/deploy
RUN go get -d -v golang.org/x/net/html \
    && go get -u github.com/golang/dep/...
COPY Gopkg.lock .
COPY Gopkg.toml .
RUN dep ensure --vendor-only
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o deploy .

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=0 /go/src/github.com/gazure/deploy/deploy .
COPY templates ./templates

ENV AWS_ACCESS_KEY_ID ""
ENV AWS_SECRET_ACCESS_KEY ""
ENV AWS_DEFAULT_REGION "us-west-2"

CMD ["./deploy"]
