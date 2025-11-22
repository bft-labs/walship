.PHONY: build docker-build docker-run release

# Build binary locally
build:
	go build ./cmd/walship

# Build Docker image locally
docker-build:
	docker build -t walship .

# Run Docker container (example with dummy env vars)
docker-run:
	docker run --rm \
		-e WALSHIP_REMOTE_URL=http://localhost:8080 \
		-e WALSHIP_AUTH_KEY=test \
		walship

# Create a new tag to trigger release (usage: make release v=v0.1.0)
release:
	git tag $(v)
	git push origin $(v)
