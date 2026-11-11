#!/usr/bin/env bash
# check-pr-tests.sh BASE HEAD — does a change carry the tests the project
# requires? Run by .github/workflows/pr-tests.yml on every PR; run it locally
# before opening one:
#
#   scripts/check-pr-tests.sh origin/main HEAD
#
# Rules (CLAUDE.md "Testing" and "every evaluator change is differential-tested"):
#   1. A package whose non-test Go code changed must change or add a test in
#      the same package. An internal/ package may be tested from any package
#      the PR touches (it is usually exercised through its importer).
#   2. A change to an evaluation shortcut — BGP planning, pushdown, caching,
#      snapshots, pre-bound joins, path probes, reachability, a store's
#      optional interfaces — must touch a seeded random differential test
#      (math/rand/v2 with a fixed rand.NewPCG seed). In sparql/ that is
#      TestOptimisationsDoNotChangeAnswers with its switch in opt_toggle.go.
#   3. No Test/Fuzz/Benchmark function may disappear.
#   4. No new conformance exemptions (an entry added to a *Known map) and no
#      new t.Skip other than "suite/submodule not checked out".
#
# A maintainer can waive the check for one PR with the label
# "tests-not-needed". Contributors cannot set labels, so a waiver is always a
# maintainer's decision. Portable to bash 3.2 (macOS): no associative arrays.
set -euo pipefail

base=${1:?usage: check-pr-tests.sh BASE HEAD}
head=${2:?usage: check-pr-tests.sh BASE HEAD}
mb=$(git merge-base "$base" "$head")

fail=0
err() { echo "::error::$*"; fail=1; }

files=$(git diff --name-only "$mb" "$head" -- '*.go')                   # incl. deleted
present=$(git diff --name-only --diff-filter=d "$mb" "$head" -- '*.go') # not deleted

is_test() { [[ $1 == *_test.go ]]; }
# Code that needs no test of its own: examples, test data, doc.go, generated
# files, and the W3C statistics tool.
exempt() {
	case $1 in
	examples/* | */testdata/* | testdata/* | doc.go | */doc.go | internal/w3cstat/*) return 0 ;;
	esac
	git show "$head:$1" 2>/dev/null | head -5 | grep -q '^// Code generated .* DO NOT EDIT\.$'
}

code_files=""
test_files=""
for f in $files; do
	if is_test "$f"; then
		test_files+="$f"$'\n'
	elif ! exempt "$f"; then
		code_files+="$f"$'\n'
	fi
done

# --- 1. every changed package has a changed test -------------------------
code_pkgs=$(printf '%s' "$code_files" | xargs -n1 dirname 2>/dev/null | sort -u || true)
test_pkgs=$(printf '%s' "$test_files" | xargs -n1 dirname 2>/dev/null | sort -u || true)
for d in $(comm -23 <(echo "$code_pkgs") <(echo "$test_pkgs")); do
	if [[ $d == internal/* && -n $test_files ]]; then
		continue
	fi
	err "$d: Go code changed but no _test.go in $d was added or changed. Add a test that fails without the change."
done

# --- 2. evaluation shortcuts need a differential test ---------------------
needs_diff=""
for f in $code_files; do
	case $f in
	sparql/bgp_*.go | sparql/cache.go | sparql/limit.go | sparql/opt_toggle.go | \
		sparql/prebound_join.go | sparql/snapshot.go | \
		paths/*.go | \
		store/union*.go | store/reachability.go | store/cardinality*.go | \
		store/queryable*.go | store/snapshot*.go)
		needs_diff+="$f "
		;;
	esac
done
if [[ -n $needs_diff ]]; then
	has_diff=0
	for f in $test_files; do
		echo "$present" | grep -qx "$f" || continue
		src=$(git show "$head:$f")
		if grep -q 'math/rand/v2' <<<"$src" && grep -q 'rand\.NewPCG(' <<<"$src"; then
			has_diff=1
			break
		fi
	done
	if ((!has_diff)); then
		err "evaluation shortcut changed (${needs_diff% }) but no seeded random differential test (math/rand/v2 with rand.NewPCG) was added or changed. Run the same queries with the change on and off and require equal multisets; the change needs an off switch for that. In sparql/ extend TestOptimisationsDoNotChangeAnswers and add the switch to opt_toggle.go."
	fi
fi

# --- 3. test functions must not disappear ---------------------------------
test_funcs() {
	git grep -h -E '^func (Test|Fuzz|Benchmark)[A-Z_0-9]' "$1" -- '*_test.go' |
		sed -E 's/^func ([A-Za-z0-9_]+).*/\1/' | sort -u
}
lost=$(comm -23 <(test_funcs "$mb") <(test_funcs "$head") | tr '\n' ' ')
if [[ -n ${lost// /} ]]; then
	err "test functions removed or renamed: ${lost}— keep the name when a test moves; if one is obsolete, say why in the PR and ask a maintainer for the tests-not-needed label."
fi

# --- 4. no new exemptions or skips ----------------------------------------
added=""
for f in $test_files; do
	echo "$present" | grep -qx "$f" || continue
	d=$(git diff -U0 "$mb" "$head" -- "$f" | grep -E '^\+[^+]' || true)
	[[ -z $d ]] && continue
	# a new t.Skip, unless it is the usual "suite not checked out" guard
	s=$(grep -E 't\.Skip(f|Now)?\(' <<<"$d" | grep -viE 'submodule|not (found|present|checked out)|testing\.Short|-short' || true)
	# a new entry in an exemption map: "name": "reason" in a file that has a *Known map
	k=""
	if git show "$head:$f" | grep -qE '[A-Za-z]*Known *(=|:=) *map\['; then
		k=$(grep -E '^\+[[:space:]]*"[^"]+"[[:space:]]*:[[:space:]]*"' <<<"$d" || true)
	fi
	for line in "$s" "$k"; do
		if [[ -n $line ]]; then
			added+="$f:"$'\n'"$line"$'\n'
		fi
	done
done
if [[ -n $added ]]; then
	err "new test skips or conformance exemptions — they need a maintainer's agreement (label tests-not-needed, with the reason in the PR):"
	printf '%s' "$added" | sed 's/^/    /'
fi

if ((fail)); then
	echo
	echo "Test requirements not met. See CLAUDE.md \"Testing\" for the rules."
	exit 1
fi
echo "Test requirements met."
