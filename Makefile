.PHONY: test

test:
	GOCACHE=/tmp/go-build GOPATH=/tmp/gopath GOMODCACHE=/tmp/gomodcache go test ./...
