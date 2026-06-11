.PHONY: lint-fix test-race tese-race

GO_PACKAGES := ./...
GO_FILES := $(shell find . -type f -name '*.go' -not -path './bin/*')

lint-fix:
	gofmt -w $(GO_FILES)
	go vet $(GO_PACKAGES)
	go test $(GO_PACKAGES)

test-race:
	go test -race $(GO_PACKAGES)

tese-race: test-race
