build:
	CGO_ENABLED=0 go build -o watcher ./cmd/watcher/

run: build
	./watcher

test:
	go test ./...

clean:
	rm -f watcher

.PHONY: build run test clean
