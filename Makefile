VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
LDFLAGS  = -s -w \
           -X github.com/yuki0ueda/e2ee-sync/internal/version.Version=$(VERSION) \
           -X github.com/yuki0ueda/e2ee-sync/internal/version.Commit=$(COMMIT)

PLATFORMS = linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64

.PHONY: build build-all clean test vet

# Build for current OS/arch
build:
	CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o bin/e2ee-sync-setup ./cmd/setup
	CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o bin/autosync ./cmd/autosync

# Cross-compile all targets
build-all:
	@for platform in $(PLATFORMS); do \
		GOOS=$${platform%/*}; \
		GOARCH=$${platform#*/}; \
		ext=""; \
		if [ "$$GOOS" = "windows" ]; then ext=".exe"; fi; \
		arch=$$(echo $$GOARCH | sed 's/amd64/x64/'); \
		os_label=$$GOOS; \
		if [ "$$GOOS" = "windows" ]; then os_label="win"; fi; \
		if [ "$$GOOS" = "darwin" ]; then os_label="mac"; fi; \
		echo "Building $$GOOS/$$GOARCH..."; \
		CGO_ENABLED=0 GOOS=$$GOOS GOARCH=$$GOARCH \
			go build -ldflags "$(LDFLAGS)" \
			-o dist/e2ee-sync-setup-$$os_label-$$arch$$ext ./cmd/setup; \
		autosync_ldflags="$(LDFLAGS)"; \
		if [ "$$GOOS" = "windows" ]; then autosync_ldflags="$(LDFLAGS) -H windowsgui"; fi; \
		CGO_ENABLED=0 GOOS=$$GOOS GOARCH=$$GOARCH \
			go build -ldflags "$$autosync_ldflags" \
			-o dist/autosync-$$os_label-$$arch$$ext ./cmd/autosync; \
	done

test:
	go test ./...

vet:
	go vet ./...

clean:
	rm -rf bin/ dist/
