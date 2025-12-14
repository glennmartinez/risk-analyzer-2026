.PHONY: run tidy build

run:
	go run ./cmd/grok-server

tidy:
	go mod tidy

build:
	go build -o bin/grok-server ./cmd/grok-server

docker-build:
	docker build -t grok-server:latest .