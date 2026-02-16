.PHONY: build clean install package test test-integration uninstall

HOTPATCH_VERSION=$$(yq .version plugin.yaml)
HOTPATCH_PACKAGE=hotpatch-$(HOTPATCH_VERSION).tgz
HOTPATCH_BUILD_OUTPUT=hotpatch

build:
	@go build -o $(HOTPATCH_BUILD_OUTPUT) ./cmd/hotpatch

clean:
	@rm -rf $(HOTPATCH_BUILD_OUTPUT) ./package $(HOTPATCH_PACKAGE)

package: build
	@mkdir -p ./package
	@cp ./hotpatch ./package/
	@cp ./plugin.yaml ./package/
	@helm plugin package ./package --sign=false
# TODO: Signing
# 	@helm plugin package ./package --sign --key "${HOTPATCH_SIGNING_KEY_EMAIL}"

install: package
	@helm plugin install $(HOTPATCH_PACKAGE)

uninstall:
	@helm plugin uninstall hotpatch

test-flags=$(if $(TEST_FLAGS),$(TEST_FLAGS))

test:
	@go test ./... $(test-flags)

test-integration:
	@go test ./integration $(test-flags) -tags integration
