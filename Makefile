APP=jamfschool2snipeIT

.PHONY: test build snapshot

test:
	go test ./...

build:
	go build ./...

snapshot:
	goreleaser release --snapshot --clean
