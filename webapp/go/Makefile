export GO111MODULE=on

all: isucari

isucari: *.go
	go build -o isucari

protogen: *.proto
	protoc --gogofaster_out=Mgoogle/protobuf/timestamp.proto=github.com/gogo/protobuf/types:. $^