.PHONY: tests
tests:
	go test ./...

.PHONY: coverage
coverage:
	go test -coverprofile cover.out ./...
	go tool cover -html=cover.out
