VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null)
LDFLAGS := -s -w -buildid= -X main.version=$(VERSION) -X main.commit=$(COMMIT)
PLATFORMS := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64

.PHONY: build test dist clean install

build:
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o capybari ./cmd/capybari

test:
	go vet ./...
	go test -race ./...

install:
	CGO_ENABLED=0 go install -trimpath -ldflags "$(LDFLAGS)" ./cmd/capybari

# Reproducible cross-platform binaries in dist/.
dist:
	@mkdir -p dist
	@for p in $(PLATFORMS); do \
		os=$${p%/*}; arch=$${p#*/}; ext=; [ $$os = windows ] && ext=.exe; \
		echo "building $$os/$$arch"; \
		GOOS=$$os GOARCH=$$arch CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o dist/capybari_$(VERSION)_$${os}_$${arch}$$ext ./cmd/capybari || exit 1; \
	done
	@cd dist && sha256sum capybari_* > checksums.txt && cat checksums.txt

clean:
	rm -rf dist capybari
