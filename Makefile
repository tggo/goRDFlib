.PHONY: test test-verbose test-sparql lint build clean

# Run all tests with summary
test:
	@echo "=== Running all tests ==="
	@go test ./... -count=1 -timeout 120s -v 2>&1 | tee /tmp/goRDFlib-test.out | grep "^ok\|^FAIL" | grep -v "no test files"
	@echo ""
	@echo "=== Summary ==="
	@passed=$$(grep -c "^    --- PASS\|^--- PASS" /tmp/goRDFlib-test.out); \
	 failed=$$(grep -c "^    --- FAIL\|^--- FAIL" /tmp/goRDFlib-test.out || true); \
	 pkgs_ok=$$(grep "^ok" /tmp/goRDFlib-test.out | grep -cv "no test files"); \
	 pkgs_fail=$$(grep -c "^FAIL" /tmp/goRDFlib-test.out || true); \
	 echo "Packages: $$pkgs_ok passed, $$pkgs_fail failed"; \
	 echo "Tests:    $$passed passed, $$failed failed"; \
	 if [ "$$pkgs_fail" != "0" ] && [ "$$pkgs_fail" != "" ]; then exit 1; fi

# Run all tests with full verbose output
test-verbose:
	go test ./... -count=1 -timeout 120s -v

# Run only SPARQL W3C conformance tests
test-sparql:
	@echo "=== W3C SPARQL 1.1 Query Tests ==="
	@go test ./sparql/ -run TestW3C -count=1 -timeout 60s -v 2>&1 | tee /tmp/goRDFlib-sparql.out | grep "^    --- FAIL" || true
	@passed=$$(grep -c "^    --- PASS" /tmp/goRDFlib-sparql.out); \
	 failed=$$(grep -c "^    --- FAIL" /tmp/goRDFlib-sparql.out || true); \
	 echo ""; \
	 echo "W3C SPARQL 1.1: $$passed passed, $$failed failed"; \
	 if [ "$$failed" != "0" ] && [ "$$failed" != "" ]; then exit 1; fi

# Lint
lint:
	gofmt -l .
	go vet ./...

# Build
build:
	go build ./...

# Clean test cache
clean:
	go clean -testcache
