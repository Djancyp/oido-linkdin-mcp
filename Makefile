.PHONY: build clean dist

PLUGIN_NAME := oido-linkedin
BINARY := $(PLUGIN_NAME)-mcp
DIST_DIR := dist

build:
	@echo "Building $(PLUGIN_NAME) MCP server..."
	CGO_ENABLED=0 go build -o $(BINARY) .
	@echo "✓ Built: $(BINARY)"
	@ls -lh $(BINARY)

dist: build
	@mkdir -p $(DIST_DIR)
	@echo "Packaging $(PLUGIN_NAME).zip..."
	@cp oido-extension.json OIDO.md $(BINARY) $(DIST_DIR)/
	@cd $(DIST_DIR) && zip -r ../$(PLUGIN_NAME).zip .
	@echo "✓ Packaged: $(PLUGIN_NAME).zip"
	@ls -lh $(PLUGIN_NAME).zip
	@echo ""
	@echo "Next steps:"
	@echo "  1. Upload $(PLUGIN_NAME).zip via Plugins UI"
	@echo "  2. Set LINKEDIN_COOKIE in extension settings"

clean:
	rm -f $(BINARY)
	rm -rf $(DIST_DIR)
	rm -f $(PLUGIN_NAME).zip
