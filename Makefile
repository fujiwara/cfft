.PHONY: clean test

cfft: go.* *.go
	go build -o $@ cmd/cfft/main.go

clean:
	rm -rf cfft dist/

test:
	go test -v ./...

install:
	go install github.com/fujiwara/cfft/cmd/cfft

dist:
	goreleaser build --snapshot --rm-dist
