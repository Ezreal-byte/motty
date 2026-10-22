.PHONY: dev head check

dev:
	tools/dev.sh

head:
	go build -tags vivahead -o /tmp/motty-head ./cmd/motty

check:
	test -z "$$(gofmt -l .)"
	go vet ./...
	go test ./... -count=1
	go test -race ./...
