.PHONY: test test-all test-race test-w3c test-bench test-verbose test-sparql lint build clean

TMPFILE := /tmp/goRDFlib-test.out

# Run all tests (short mode) with pretty statistics
test:
	@echo "================================================"
	@echo "  rdflibgo — Full Test Suite"
	@echo "================================================"
	@echo ""
	@go test ./... -count=1 -short -timeout 300s -v 2>&1 | tee $(TMPFILE) | grep -E "^ok|^FAIL" | grep -v "no test files"
	@echo ""
	@$(MAKE) --no-print-directory _stats

# Run all tests with race detector
test-race:
	@echo "================================================"
	@echo "  rdflibgo — Full Test Suite (race detector)"
	@echo "================================================"
	@echo ""
	@go test -race ./... -count=1 -short -timeout 300s -v 2>&1 | tee $(TMPFILE) | grep -E "^ok|^FAIL" | grep -v "no test files"
	@echo ""
	@$(MAKE) --no-print-directory _stats

# Run all W3C conformance tests with detailed per-suite statistics
test-w3c:
	@echo "================================================"
	@echo "  rdflibgo — W3C Conformance Tests"
	@echo "================================================"
	@echo ""
	@total_pass=0; total_fail=0; \
	for suite in \
		"SPARQL 1.1 Query:sparql:TestW3C$$" \
		"SPARQL 1.2:sparql:TestW3CSPARQL12" \
		"SPARQL Update:sparql:TestW3CUpdate" \
		"N-Triples:nt:TestW3C" \
		"N-Quads:nq:TestW3C" \
		"Turtle:turtle:TestW3C" \
		"TriG:trig:TestW3C" \
		"RDF/XML:rdfxml:TestW3C" \
		"SHACL:shacl:TestW3C" \
		"Reasoning:reasoning:TestW3C" \
	; do \
		name=$$(echo "$$suite" | cut -d: -f1); \
		pkg=$$(echo "$$suite" | cut -d: -f2); \
		pattern=$$(echo "$$suite" | cut -d: -f3); \
		output=$$(go test ./$$pkg/ -run "$$pattern" -v -count=1 -timeout 120s 2>&1); \
		pass=$$(echo "$$output" | grep -cF -- '--- PASS' || true); \
		fail=$$(echo "$$output" | grep -cF -- '--- FAIL' || true); \
		total_pass=$$((total_pass + pass)); \
		total_fail=$$((total_fail + fail)); \
		if [ "$$fail" = "0" ]; then \
			printf "  %-22s %4d / %d  ✓\n" "$$name" "$$pass" "$$pass"; \
		else \
			printf "  %-22s %4d / %d  ✗ (%d FAILED)\n" "$$name" "$$pass" "$$((pass + fail))" "$$fail"; \
		fi; \
	done; \
	echo ""; \
	echo "------------------------------------------------"; \
	printf "  %-22s %4d / %d\n" "TOTAL" "$$total_pass" "$$((total_pass + total_fail))"; \
	echo "------------------------------------------------"; \
	if [ "$$total_fail" != "0" ]; then exit 1; fi

# Run everything: race tests + W3C conformance + benchmarks
test-all:
	@$(MAKE) --no-print-directory test-race
	@echo ""
	@$(MAKE) --no-print-directory test-w3c
	@echo ""
	@$(MAKE) --no-print-directory test-bench

# Run all benchmarks with pretty output
test-bench:
	@echo "================================================"
	@echo "  rdflibgo — Benchmarks"
	@echo "================================================"
	@echo ""
	@echo "--- Store Benchmarks ---"
	@go test ./store/... -bench=. -benchmem -run=^$$ -count=1 -timeout 300s 2>&1 | grep -E "^Benchmark|^ok" | sed 's/^/  /'
	@echo ""
	@echo "--- SPARQL Benchmarks ---"
	@go test ./sparql/ -bench=. -benchmem -run=^$$ -count=1 -timeout 120s 2>&1 | grep -E "^Benchmark|^ok" | sed 's/^/  /'
	@echo ""
	@echo "--- Term Benchmarks ---"
	@go test ./term/ -bench=. -benchmem -run=^$$ -count=1 -timeout 60s 2>&1 | grep -E "^Benchmark|^ok" | sed 's/^/  /'
	@echo ""
	@echo "--- Reasoning Benchmarks ---"
	@go test ./reasoning/ -bench=. -benchmem -run=^$$ -count=1 -timeout 60s 2>&1 | grep -E "^Benchmark|^ok" | sed 's/^/  /'
	@echo ""
	@echo "--- SHACL Benchmarks ---"
	@go test ./shacl/ -bench=. -benchmem -run=^$$ -count=1 -timeout 60s 2>&1 | grep -E "^Benchmark|^ok" | sed 's/^/  /'
	@echo ""
	@echo "--- Full Application Benchmarks ---"
	@go test ./benchmarks/ -bench=. -benchmem -run=^$$ -count=1 -timeout 300s 2>&1 | grep -E "^Benchmark|^ok" | sed 's/^/  /'
	@echo ""

# Run all tests with full verbose output
test-verbose:
	go test ./... -count=1 -timeout 300s -v

# Run only SPARQL W3C conformance tests
test-sparql:
	@echo "=== W3C SPARQL 1.1 Query Tests ==="
	@go test ./sparql/ -run TestW3C -count=1 -timeout 60s -v 2>&1 | tee $(TMPFILE) | grep "^    --- FAIL" || true
	@passed=$$(grep -cF -- '--- PASS' $(TMPFILE)); \
	 failed=$$(grep -cF -- '--- FAIL' $(TMPFILE) || true); \
	 echo ""; \
	 echo "W3C SPARQL 1.1: $$passed passed, $$failed failed"; \
	 if [ "$$failed" != "0" ] && [ "$$failed" != "" ]; then exit 1; fi

# Lint
lint:
	@echo "=== go vet ==="
	@go vet ./...
	@echo "go vet: OK"
	@echo ""
	@if command -v golangci-lint >/dev/null 2>&1; then \
		echo "=== golangci-lint ==="; \
		golangci-lint run ./... 2>&1; \
	elif command -v staticcheck >/dev/null 2>&1; then \
		echo "=== staticcheck ==="; \
		staticcheck ./...; \
	else \
		echo "No linter found (install golangci-lint or staticcheck)"; \
	fi

# Build
build:
	go build ./...

# Clean test cache
clean:
	go clean -testcache

# Internal: print test statistics from $(TMPFILE)
_stats:
	@echo "================================================"
	@echo "  Summary"
	@echo "================================================"
	@passed=$$(grep -cF -- '--- PASS' $(TMPFILE) || true); \
	 failed=$$(grep -cF -- '--- FAIL' $(TMPFILE) || true); \
	 skipped=$$(grep -cF -- '--- SKIP' $(TMPFILE) || true); \
	 pkgs_ok=$$(grep "^ok" $(TMPFILE) | grep -cv "no test files" || true); \
	 pkgs_fail=$$(grep -c "^FAIL" $(TMPFILE) || true); \
	 echo ""; \
	 printf "  Packages:  %d passed" "$$pkgs_ok"; \
	 if [ "$$pkgs_fail" != "0" ] && [ -n "$$pkgs_fail" ]; then printf ", %d FAILED" "$$pkgs_fail"; fi; \
	 echo ""; \
	 printf "  Tests:     %d passed" "$$passed"; \
	 if [ "$$failed" != "0" ] && [ -n "$$failed" ]; then printf ", %d FAILED" "$$failed"; fi; \
	 if [ "$$skipped" != "0" ] && [ -n "$$skipped" ]; then printf ", %d skipped" "$$skipped"; fi; \
	 echo ""; \
	 echo ""; \
	 if [ "$$pkgs_fail" != "0" ] && [ -n "$$pkgs_fail" ]; then exit 1; fi
