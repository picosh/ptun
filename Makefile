build:
	go build -o ./build/example ./cmd/example
.PHONY: build

tunnel:
	ssh -L 8081:localhost:3000 \
		-p 2222 \
		-o UserKnownHostsFile=/dev/null \
		-o StrictHostKeyChecking=no \
		-N localhost
.PHONY: tunnel
