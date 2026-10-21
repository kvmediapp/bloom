.PHONY: test run docker

test:
	cd server && go vet ./... && go test ./...

# Example: make run MEDIA=/path/to/media
run:
	cd server && BLOOM_MEDIA_DIR=$(MEDIA) BLOOM_CACHE_DIR=/tmp/bloom-cache go run ./cmd/bloomd

docker:
	docker build -t bloom-server server/
