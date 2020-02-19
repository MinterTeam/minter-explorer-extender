FROM golang:1.13.5

WORKDIR /app

COPY ./ /app

RUN apt-get install -y make gcc

RUN go mod download

RUN go get github.com/githubnemo/CompileDaemon

ENTRYPOINT CompileDaemon --exclude-dir=.git --build="go build -o ./builds/linux/extender ./cmd/extender.go" -command="./builds/linux/extender"
