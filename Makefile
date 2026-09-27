export COMPONENT := laserlabel

BINARIES := $(notdir $(patsubst %/,%,$(wildcard cmd/*/)))

VERSION       ?= 0.1.0
ARCHITECTURES := amd64 arm64
MAINTAINER    := $(shell git config user.name) <$(shell git config user.email)>
export MAINTAINER

BUILD_TIME  := $(shell date +"%Y-%m-%dT%H:%M")
COMMITHASH  := $(shell git rev-parse --verify HEAD 2>/dev/null || echo "none")
BUILT_BY    := $(shell whoami)
LINKER_OPTS := -X github.com/borud/laserlabel/pkg/appinfo.CommitHash=$(COMMITHASH) \
	-X github.com/borud/laserlabel/pkg/appinfo.BuildTime=$(BUILD_TIME) \
	-X github.com/borud/laserlabel/pkg/appinfo.TaggedVersion=$(VERSION) \
	-X github.com/borud/laserlabel/pkg/appinfo.BuiltBy=$(BUILT_BY)

.PHONY: all
.PHONY: web
.PHONY: build
.PHONY: $(BINARIES)
.PHONY: vet
.PHONY: lint
.PHONY: staticcheck
.PHONY: test
.PHONY: race-test
.PHONY: cleantest
.PHONY: clean
.PHONY: fullclean
.PHONY: install-deps
.PHONY: build-linux
.PHONY: package
.PHONY: package-amd64
.PHONY: package-arm64

all: web vet lint staticcheck test build

build: $(BINARIES)

$(BINARIES):
	@echo "*** building $@"
	@cd cmd/$@ && CGO_ENABLED=0 go build -ldflags "$(LINKER_OPTS)" -trimpath -o ../../bin/$@

web:
	@echo "*** $@"
	@cd web && elm make src/Main.elm --optimize --output=dist/elm.js > /dev/null
	@cp -R web/public/. web/dist/

vet:
	@echo "*** $@"
	@go vet ./...

lint:
	@echo "*** $@"
	@revive ./...

staticcheck:
	@echo "*** $@"
	@staticcheck ./...

test:
	@echo "*** $@"
	@go test -timeout 2m ./...

race-test:
	@echo "*** $@"
	@go test -race -timeout 2m ./...

cleantest:
	@echo "*** $@"
	@go clean -testcache

build-linux: web
	@for arch in $(ARCHITECTURES); do \
		for bin in $(BINARIES); do \
			echo "*** building $$bin (linux/$$arch)" && \
			CGO_ENABLED=0 GOOS=linux GOARCH=$$arch go build -ldflags "$(LINKER_OPTS)" -trimpath -o bin/$$bin-$$arch-linux ./cmd/$$bin || exit 1; \
		done \
	done

package-amd64: build-linux
	@export PACKAGE_VERSION=$(VERSION) GOARCH=amd64 && \
		echo "*** packaging $(COMPONENT) $$PACKAGE_VERSION ($$GOARCH)" && \
		mkdir -p dist && \
		sed "s/\$${GOARCH}/$$GOARCH/g" deb/nfpm.yaml > dist/nfpm-$$GOARCH.yaml && \
		nfpm pkg --packager deb --config dist/nfpm-$$GOARCH.yaml --target dist/

package-arm64: build-linux
	@export PACKAGE_VERSION=$(VERSION) GOARCH=arm64 && \
		echo "*** packaging $(COMPONENT) $$PACKAGE_VERSION ($$GOARCH)" && \
		mkdir -p dist && \
		sed "s/\$${GOARCH}/$$GOARCH/g" deb/nfpm.yaml > dist/nfpm-$$GOARCH.yaml && \
		nfpm pkg --packager deb --config dist/nfpm-$$GOARCH.yaml --target dist/

package: package-amd64 package-arm64

clean:
	@echo "*** cleaning"
	@rm -rf bin
	@rm -rf dist
	@find web/dist -mindepth 1 ! -name .gitkeep -delete

fullclean: clean cleantest

install-deps:
	@go install github.com/mgechev/revive@latest
	@go install honnef.co/go/tools/cmd/staticcheck@latest
