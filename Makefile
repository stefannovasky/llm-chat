run:
	go run ./cmd/llm-chat

build:
	go build -o bin/llm-chat ./cmd/llm-chat

.PHONY: run build
