PREFIX ?= /usr/local
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

.PHONY: build install clean

build:
	go build -ldflags "-X main.version=$(VERSION)" -o bin/anvil ./cmd/anvil

install: build
	install -d $(PREFIX)/bin
	install -m 755 bin/anvil $(PREFIX)/bin/anvil

clean:
	rm -rf bin
