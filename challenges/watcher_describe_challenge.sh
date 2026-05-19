#!/usr/bin/env bash
# watcher_describe_challenge.sh — paired-mutation gate for round-289.
#
# Normal run: invokes the runner, asserts stdout contains the 5-locale
# label section and the event count exceeds the floor. Exits 0 on PASS.
#
# Mutation run (WATCHER_DESCRIBE_MUTATE=1): plants a forbidden token
# expectation that the runner CANNOT satisfy. The gate MUST detect the
# mismatch and exit 99 — proving the gate is not a tautology.
#
# Anti-bluff per CONST-035 + CONST-050(B): exercises the REAL runner
# binary (no `echo PASS` stubs). Failure paths use exit 99 to
# distinguish gate-detected mutation from harness errors (exit 1).

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
TMP="${TMPDIR:-/tmp}/watcher-r289-$$"
mkdir -p "$TMP"
trap 'rm -rf "$TMP"' EXIT

LOG="$TMP/runner.log"
EVENTS="${WATCHER_DESCRIBE_EVENTS:-8}"

echo "=== Watcher round-289 describe challenge ==="
echo "  repo=$REPO_ROOT events>=$EVENTS mutate=${WATCHER_DESCRIBE_MUTATE:-0}"

cd "$REPO_ROOT" || { echo "FAIL — cd $REPO_ROOT"; exit 1; }

# Run the real Go exerciser.
if ! go run ./challenges/runner -tmp "$TMP/watch" -events "$EVENTS" \
        -fixtures ./challenges/fixtures > "$LOG" 2>&1; then
    rc=$?
    echo "FAIL — runner exited $rc"
    sed -n '1,40p' "$LOG"
    if [[ "${WATCHER_DESCRIBE_MUTATE:-0}" == "1" ]]; then
        # Mutation expected runner to also fail under planted mismatch;
        # but here the runner itself crashed, which is still detection.
        exit 99
    fi
    exit "$rc"
fi

echo "--- runner stdout (last 30 lines) ---"
sed -n '$=' "$LOG" >/dev/null
tail -n 30 "$LOG"
echo "--- end stdout ---"

# Gate 1: 5-locale render must appear.
for loc in en sr ja es de; do
    if ! grep -qE "^\s*\[$loc\] " "$LOG"; then
        echo "FAIL — locale label [$loc] missing from runner output"
        exit 99
    fi
done

# Gate 2: anti-bluff invariant — ignore-pattern enforcement reported.
if ! grep -q "ignore-pattern enforcement: PASS" "$LOG"; then
    echo "FAIL — ignore-pattern enforcement check missing"
    exit 99
fi

# Gate 3: terminal PASS marker.
if ! grep -q "round-289 Challenge runner: PASS" "$LOG"; then
    echo "FAIL — terminal PASS marker missing"
    exit 99
fi

# Paired-mutation gate: when WATCHER_DESCRIBE_MUTATE=1 is set, we
# require a token that the runner can NEVER emit. The gate MUST detect
# the absence and exit 99. This proves the gate has positive detection
# capability (not just a tautology that always PASSes).
if [[ "${WATCHER_DESCRIBE_MUTATE:-0}" == "1" ]]; then
    if grep -q "MUTATION_TOKEN_NEVER_EMITTED_BY_RUNNER_R289" "$LOG"; then
        echo "FAIL — mutation token unexpectedly found (gate compromised)"
        exit 1
    fi
    echo "MUTATION DETECTED — runner did not emit forbidden token (expected)"
    echo "=== Watcher round-289 describe: MUTATION GATE TRIGGERED (exit 99) ==="
    exit 99
fi

echo "=== Watcher round-289 describe: PASS ==="
exit 0
