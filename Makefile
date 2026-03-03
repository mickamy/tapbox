BUILD_DIR = bin
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS = -ldflags "-X main.version=$(VERSION)"

.PHONY: all build build-tap build-tapd install uninstall clean test lint demo

all: build

build: build-tapbox

build-tapbox:
	@echo "Building tapbox..."
	go build $(LDFLAGS) -o $(BUILD_DIR)/tapbox .

install:
	@echo "Installing tapbox..."
	@bin_dir=$$(go env GOBIN); \
	if [ -z "$$bin_dir" ]; then \
		bin_dir=$$(go env GOPATH)/bin; \
	fi; \
	mkdir -p "$$bin_dir"; \
	echo "Installing to $$bin_dir"; \
	go build $(LDFLAGS) -o "$$bin_dir/tapbox" .

uninstall:
	@echo "Uninstalling tapbox..."
	@bin_dir=$$(go env GOBIN); \
	if [ -z "$$bin_dir" ]; then \
		bin_dir=$$(go env GOPATH)/bin; \
	fi; \
	rm -f "$$bin_dir/tapbox"

clean:
	@echo "Cleaning up..."
	rm -rf $(BUILD_DIR)

test:
	go test -race ./...

lint:
	@command -v golangci-lint >/dev/null 2>&1 || { \
		echo "golangci-lint is not installed"; \
		exit 1; \
	}
	golangci-lint run
	cd example && golangci-lint run

demo:
	rm -rf ./demo/output
	cd demo && npm ci && npx playwright install chromium && npx tsx record.ts
	cd demo && ffmpeg -y -i output/demo.webm \
		-vf "fps=12,scale=1280:-1:flags=lanczos,split[s0][s1];[s0]palettegen=max_colors=128[p];[s1][p]paletteuse=dither=bayer:bayer_scale=3" \
		-loop 0 output/demo.gif
	cp ./demo/output/demo.gif ./docs/demo.gif
