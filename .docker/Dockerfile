FROM golang:1.17-alpine as builder
ARG VERSION=${VERSION}
WORKDIR /app
COPY . /app
RUN apk add --no-cache make gcc musl-dev linux-headers
RUN go mod tidy
RUN go build -ldflags="-X 'github.com/MinterTeam/minter-explorer-extender/v2/core.Version=$VERSION'" -o ./builds/linux/extender ./cmd/extender.go

FROM alpine:3.14
COPY --from=builder /app/builds/linux/extender /usr/bin/extender
RUN addgroup minteruser && adduser -D -h /minter -G minteruser minteruser
USER minteruser
WORKDIR /minter
ENTRYPOINT ["/usr/bin/extender"]
CMD ["extender"]
