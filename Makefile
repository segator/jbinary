BINARY_NAME=jbinary
MAIN_PACKAGE_PATH=.
OUTPUT_DIR=dist
RELEASE_VERSION ?= dev
PLATFORMS=linux/amd64 linux/arm64 windows/amd64 windows/arm64


.PHONY: all
all: build


.PHONY: build
build:
	@echo "Starting build for version: $(VERSION)..."
	@mkdir -p $(OUTPUT_DIR)
	@$(foreach platform,$(PLATFORMS), $(call build_platform,$(platform)))
	@echo "Build complete. Binaries are in $(OUTPUT_DIR)/"

# A helper function to build for a single platform
define build_platform
	$(eval parts = $(subst /, ,$(1)))
	$(eval os = $(word 1,$(parts)))
	$(eval arch = $(word 2,$(parts)))
	@echo "--> Building for $(os)/$(arch)..."
	$(eval output_name = "$(OUTPUT_DIR)/$(BINARY_NAME)-$(RELEASE_VERSION)-$(os)-$(arch)")
	$(if $(findstring windows,$(os)),$(eval output_name = "$(output_name).exe"))
	@GOOS=$(os) GOARCH=$(arch) go build -v -o $(output_name) $(MAIN_PACKAGE_PATH)
endef

# Target to clean up the build artifacts
.PHONY: clean
clean:
	@echo "Cleaning up build artifacts..."
	@rm -rf $(OUTPUT_DIR)
	@echo "Done."