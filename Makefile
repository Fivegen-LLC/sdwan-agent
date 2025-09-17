BIN_NAME=sdwan-agent

build:
	@echo "Building for default platform..."
	env CGO_ENABLED=0 go build -trimpath -o ./bin/${BIN_NAME} ./cmd/app/.
	@echo "Done!"

build-linux:
	@echo "Building for Linux..."
	env GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -o ./bin/${BIN_NAME} ./cmd/app/.
	@echo "Done!"

lint:
	golangci-lint run

mocks:
	go tool mockery --config=.mockery.yaml
