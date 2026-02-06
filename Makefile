BINARY    := filehasher
VERSION   := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS   := -s -w -X main.version=$(VERSION)
BUILD_DIR := build

.PHONY: all build clean test fmt vet linux release

all: build

build:
	CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o $(BINARY) ./cmd/

linux:
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY) ./cmd/

clean:
	rm -f $(BINARY)
	rm -rf $(BUILD_DIR)

test:
	go test ./...

fmt:
	go fmt ./...

vet:
	go vet ./...

# Build a release tarball for Unraid (linux/amd64, static, stripped).
# Produces build/filehasher-linux-amd64.tar.gz containing a single "filehasher" binary.
release: clean
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY) ./cmd/
	tar czf $(BUILD_DIR)/$(BINARY)-linux-amd64.tar.gz -C $(BUILD_DIR) $(BINARY)
	@echo "Built: $(BUILD_DIR)/$(BINARY)-linux-amd64.tar.gz"
	@ls -lh $(BUILD_DIR)/$(BINARY)-linux-amd64.tar.gz
