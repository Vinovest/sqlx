.ONESHELL:
SHELL = /bin/sh
.SHELLFLAGS = -ec

BASE_PACKAGE := github.com/vinovest/sqlx

tooling:
	go install honnef.co/go/tools/cmd/staticcheck@v0.6.1
	go install golang.org/x/vuln/cmd/govulncheck@v1.1.4
	go install golang.org/x/tools/cmd/goimports@v0.20.0

has-changes:
	git diff --exit-code --quiet HEAD --

lint:
	go vet ./...
	staticcheck -checks=all ./...

fmt:
	go fmt ./...
	goimports -l -w .

vuln-check:
	govulncheck ./...

test-race:
	go test -v -race -count=1 ./...

update-dependencies:
	go get -u -t -v ./...
	go mod tidy

test-examples:
	for dir in $$(find examples -type d); do \
		if [ -f $$dir/main.go ]; then \
			echo "Building and running $$dir/main.go"; \
			go build -o $$dir/main $$dir/main.go && $$dir/main; \
		fi \
	done
