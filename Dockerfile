#FROM golang:1.13.7-alpine as builder
#
#WORKDIR /app
#COPY ./ /app
#RUN apk add --no-cache make gcc musl-dev linux-headers
#RUN go mod download
#RUN go build -o ./builds/linux/extender ./cmd/extender.go
#
#FROM alpine:3.7
#
#COPY --from=builder /app/builds/linux/extender /usr/bin/extender
#RUN addgroup minteruser && adduser -D -h /minter -G minteruser minteruser
#USER minteruser
#WORKDIR /minter
#ENTRYPOINT ["/usr/bin/extender"]
#CMD ["extender"]

FROM golang:1.13.5

WORKDIR /app

COPY ./ /app

RUN go mod download

RUN go get github.com/githubnemo/CompileDaemon

ENTRYPOINT CompileDaemon --exclude-dir=.git --build="go build -o ./builds/linux/extender ./cmd/extender.go" --command=./builds/linux/extender
