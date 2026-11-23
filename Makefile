# Makefile for the tagged-enums Go toolchain.
#
# Builds this tree, then installs a self-contained SDK copy to PREFIX.
# GOROOT_FINAL is unsupported in this tree, so the installed `go` is wrapped
# to pin GOROOT and GOTOOLCHAIN=local for every invocation.

PREFIX ?= $(HOME)/sdk/go1.28-tagged_enums
BUILD_ROOT := $(CURDIR)

.PHONY: all install verify

## all: build the toolchain in this tree.
all:
	cd src && ./make.bash

## install: build, copy the toolchain tree to PREFIX, and wrap bin/go.
install: all
	@test -n "$(PREFIX)" || { echo "PREFIX is empty" >&2; exit 1; }
	@echo "Installing Go toolchain from $(BUILD_ROOT) to $(PREFIX)"
	mkdir -p "$(PREFIX)"
	rsync -a bin pkg src lib api go.env VERSION "$(PREFIX)/"
	mv -f "$(PREFIX)/bin/go" "$(PREFIX)/bin/go.real"
	printf '%s\n' \
		'#!/bin/sh' \
		'GOROOT="$(PREFIX)" GOTOOLCHAIN=local exec "$(PREFIX)/bin/go.real" "$$@"' \
		> "$(PREFIX)/bin/go"
	chmod +x "$(PREFIX)/bin/go"
	@echo ""
	@echo "Installed and wrapped $(PREFIX)/bin/go."

## verify: check the installed SDK and its compiler version.
verify:
	@$(PREFIX)/bin/go version
	@$(PREFIX)/bin/go env GOROOT GOTOOLCHAIN
	@$(PREFIX)/bin/go tool compile -V=full
