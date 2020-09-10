FROM golang:1.13.7-alpine as builder

WORKDIR /app
COPY . /app
RUN apk add --no-cache make gcc musl-dev linux-headers
RUN go mod download
RUN go build -ldflags="-X 'github.com/MinterTeam/minter-explorer-extender/v2/core.Version=$VERSION' -o ./builds/linux/extender ./cmd/extender.go

FROM alpine:3.7

COPY --from=builder /app/builds/linux/extender /usr/bin/extender
RUN addgroup minteruser && adduser -D -h /minter -G minteruser minteruser
USER minteruser
WORKDIR /minter
ENTRYPOINT ["/usr/bin/extender"]
CMD ["extender"]
