fmt:
	go fmt ./...
.PHONY: fmt

lint:
	golangci-lint run -E goimports -E godot --timeout 10m
.PHONY: lint

build-example:
	go build -o ./build/example ./cmd/example
.PHONY: build

build-docker:
	go build -o ./build/docker ./cmd/docker
.PHONY: build-docker

tunnel:
	ssh -L 8081:localhost:3000 \
		-p 2222 \
		-o UserKnownHostsFile=/dev/null \
		-o StrictHostKeyChecking=no \
		-N localhost
.PHONY: tunnel
