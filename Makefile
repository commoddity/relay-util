build-windows:
	GOOS=windows GOARCH=amd64 go build -o bin/relay-util.exe main.go
build-linux:
	GOOS=linux GOARCH=amd64 go build -o bin/relay-util main.go
build-mac:
	GOOS=darwin GOARCH=amd64 go build -o bin/relay-util main.go