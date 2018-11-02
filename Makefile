# Makefile used to create packages for copy. It doesn't assume that the code is
# inside a GOPATH, and always copy the files into a new workspace to get the
# work done. Go tools doesn't reliably work with symbolic links.
#
# For historical purposes, it also works in a development environment when the
# repository is already inside a GOPATH.
.NOTPARALLEL:

GO_PACKAGE_PREFIX := github.com/matthewrsj/copy

.PHONY: gopath

# Strictly speaking we should check if it the directory is inside an
# actual GOPATH, but the directory structure matching is likely enough.
ifeq (,$(findstring ${GO_PACKAGE_PREFIX},${CURDIR}))
LOCAL_GOPATH := ${CURDIR}/.gopath
export GOPATH := ${LOCAL_GOPATH}
gopath:
	@rm -rf ${LOCAL_GOPATH}/src
	@mkdir -p ${LOCAL_GOPATH}/src/${GO_PACKAGE_PREFIX}
	@cp -af * ${LOCAL_GOPATH}/src/${GO_PACKAGE_PREFIX}
	@echo "Prepared a local GOPATH=${GOPATH}"
else
LOCAL_GOPATH :=
GOPATH ?= ${HOME}/go
gopath:
	@echo "Code already in existing GOPATH=${GOPATH}"
endif

.PHONY: check

.DEFAULT_GOAL := check

check: gopath
	go test -cover ${GO_PACKAGE_PREFIX}/...

# TODO: since Go 1.10 we have support for passing multiple packages
# to coverprofile. Update this to work on all packages.
.PHONY: checkcoverage
checkcoverage: gopath
	go test -cover . -coverprofile=coverage.out
	go tool cover -html=coverage.out

.PHONY: lint
lint: gopath
	@gometalinter.v2 --deadline=10m --tests --vendor --disable-all \
	--enable=misspell \
	--enable=vet \
	--enable=ineffassign \
	--enable=gofmt \
	--enable=gocyclo --cyclo-over=15 \
	--enable=golint \
	--enable=deadcode \
	--enable=varcheck \
	--enable=structcheck \
	--enable=unused \
	--enable=vetshadow \
	--enable=errcheck \
	./...

clean:
ifeq (,${LOCAL_GOPATH})
	go clean -i -x
else
	rm -rf ${LOCAL_GOPATH}
endif
	rm -f copy-*.tar.gz

release:
	@if [ ! -d .git ]; then \
		echo "Release needs to be used from a git repository"; \
		exit 1; \
	fi
	@VERSION=0.0.1
	if [ -z "$$VERSION" ]; then \
		exit 1; \
	fi; \
	git archive --format=tar.gz --verbose -o copy-$$VERSION.tar.gz HEAD --prefix=copy-$$VERSION/
