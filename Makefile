.PHONY: help dev stop fmt vet check

# Default target
help:
	@echo "cclist - ClaudeCode List"
	@echo ""
	@echo "Available targets:"
	@echo "  make dev   - Start development server (testdata/)"
	@echo "  make stop  - Stop development server"
	@echo "  make fmt   - Format code with goimports"
	@echo "  make vet   - Run go vet for compile checks"
	@echo "  make check - Run all checks"

# Start development server (run in testdata/)
# Build clean environment with reset.sh before starting
# Set shutdown token to fixed value for debugging
dev:
	cd testdata && ./reset.sh && CCLIST_SHUTDOWN_TOKEN=debug-token-12345 go run ..

# Stop development server
stop:
	@curl -X POST http://127.0.0.1:12012/api/shutdown \
		-H 'X-Shutdown-Token: debug-token-12345' \
		2>/dev/null && echo "Server stopped successfully" || echo "Server not running or failed to stop"

# Format code
fmt:
	goimports -local github.com/daisuzu/cclist -w .

# Compile check
vet:
	go vet ./...

# Run all checks
check: fmt vet
	go run golang.org/x/tools/gopls/internal/analysis/modernize/cmd/modernize@latest ./...
	go run golang.org/x/tools/cmd/deadcode@latest ./...
	staticcheck ./...
