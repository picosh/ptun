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
	ssh -L 0.0.0.0:8443:localhost:8080 \
		-p 2222 \
		-o UserKnownHostsFile=/dev/null \
		-o StrictHostKeyChecking=no \
		-N \
		localhost
.PHONY: tunnel

tunnel-prod:
	ssh -L 0.0.0.0:8443:localhost:80 -N imgs.sh
	# docker pull ubuntu
	# docker tag ubuntu localhost:5000/ubuntu
	# docker push localhost:5000/ubuntu
.PHONY: tunnel
