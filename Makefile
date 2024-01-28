build-windows:
	GOOS=windows GOARCH=amd64 go build -o bin/relay-util.exe main.go
build-linux:
	GOOS=linux GOARCH=amd64 go build -o bin/relay-util main.go
build-mac:
	GOOS=darwin GOARCH=amd64 go build -o bin/relay-util main.go

# This target install pre-commit to the repo and should be run only once, after cloning the repo for the first time.
init-pre-commit:
	wget https://github.com/pre-commit/pre-commit/releases/download/v2.20.0/pre-commit-2.20.0.pyz;
	python3 pre-commit-2.20.0.pyz install;
	go install golang.org/x/tools/cmd/goimports@latest;
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest;
	go install -v github.com/go-critic/go-critic/cmd/gocritic@latest;
	python3 pre-commit-2.20.0.pyz run --all-files;
	rm pre-commit-2.20.0.pyz.*;
