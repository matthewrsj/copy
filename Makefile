# Makefile used to create packages for copy. It doesn't assume that the code is
# inside a GOPATH, and always copy the files into a new workspace to get the
# work done. Go tools doesn't reliably work with symbolic links.
#
# For historical purposes, it also works in a development environment when the
# repository is already inside a GOPATH.
.NOTPARALLEL:

GO_PACKAGE_PREFIX := github.com/matthewrsj/copy

.PHONY: check

.DEFAULT_GOAL := check

check:
	go test -cover ${GO_PACKAGE_PREFIX}/...

# TODO: since Go 1.10 we have support for passing multiple packages
# to coverprofile. Update this to work on all packages.
.PHONY: checkcoverage
checkcoverage:
	go test -cover . -coverprofile=coverage.out
	go tool cover -html=coverage.out

.PHONY: lint
lint:
	@golangci-lint run ./...

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
