#!/bin/sh
# shellcheck disable=SC1090  # the helper path is computed at run time, from this script's own location
# Tests for .docker/cscli-hub-mirror.sh. No Docker and no network: a fake cscli on PATH records each call and
# answers from FAKE_MODE. POSIX sh only, so the same file runs under dash, busybox sh and bash:
#   sh scripts/cscli_hub_mirror_test.sh
#   dash scripts/cscli_hub_mirror_test.sh
#   busybox sh scripts/cscli_hub_mirror_test.sh
# Exit status 0 when every check passes.

REPO_DIR=$(cd "$(dirname "$0")/.." && pwd)
HELPER="$REPO_DIR/.docker/cscli-hub-mirror.sh"
ENTRYPOINT="$REPO_DIR/.docker/docker-entrypoint.sh"

WORK=$(mktemp -d "${TMPDIR:-/tmp}/cscli-hub-mirror-test.XXXXXX") || exit 1
trap 'rm -rf "$WORK"' EXIT INT TERM

FAKEBIN="$WORK/bin"
mkdir -p "$FAKEBIN"
TEMPLATE='https://raw.githubusercontent.com/crowdsecurity/hub/%s/%s'
TEMPLATE_LINE="  __hub_url_template__: \"$TEMPLATE\""

# Fake cscli. `-c <config>` selects a config; the fake logs one line per call (cfg, whether the derived config
# carries the template, whether its .local copy exists, and the arguments). FAKE_MODE decides the answers:
#   live_ok      the live config succeeds
#   live_net     the live config fails with a network error; the derived config succeeds
#   live_bad     the live config fails with an item error; the derived config succeeds
#   live_hang    the live config never answers; the derived config succeeds
#   derived_net  the derived config fails with a network error; the live config succeeds
#   both_net     both fail with a network error
cat >"$FAKEBIN/cscli" <<'FAKE'
#!/bin/sh
cfg=""
if [ "$1" = "-c" ]; then
    cfg=$2
    shift 2
fi
tpl=0
localfile=0
if [ -n "$cfg" ]; then
    if grep -Fqx '  __hub_url_template__: "https://raw.githubusercontent.com/crowdsecurity/hub/%s/%s"' "$cfg"; then
        tpl=1
    fi
    if [ -e "$cfg.local" ]; then
        localfile=1
    fi
fi
echo "cfg=${cfg:-live} tpl=$tpl local=$localfile args=$*" >>"$FAKE_LOG"
net="failed http request for https://cdn-hub.crowdsec.net/x: dial tcp 127.0.0.1:443: connection refused"
bad="item not found: crowdsecurity/nope"
if [ -z "$cfg" ]; then
    case "$FAKE_MODE" in
        live_ok) echo ok; exit 0 ;;
        live_net) echo "$net"; exit 1 ;;
        live_bad) echo "$bad"; exit 1 ;;
        live_hang) exec sleep 30 ;;
        derived_net) echo ok; exit 0 ;;
        both_net) echo "$net"; exit 1 ;;
    esac
else
    case "$FAKE_MODE" in
        live_ok|live_net|live_bad|live_hang) echo ok; exit 0 ;;
        derived_net|both_net) echo "$net"; exit 1 ;;
    esac
fi
FAKE
chmod +x "$FAKEBIN/cscli"
PATH="$FAKEBIN:$PATH"
export PATH

PASS=0
FAILS=0
pass() {
    PASS=$((PASS + 1))
    echo "ok - $1"
}
fail() {
    FAILS=$((FAILS + 1))
    echo "not ok - $1"
}
# check <name> <expected> <actual>
check() {
    if [ "$2" = "$3" ]; then
        pass "$1"
    else
        fail "$1 (expected [$2], got [$3])"
    fi
}
# check_file <name> <expected file> <actual file>
check_file() {
    if cmp -s "$2" "$3"; then
        pass "$1"
    else
        fail "$1 (content differs: $(diff "$2" "$3" | head -n 12 | tr '\n' '|'))"
    fi
}

# make_live <dir>: a live cscli config as the entrypoint leaves it (hub_branch pinned).
make_live() {
    mkdir -p "$1"
    cat >"$1/config.yaml" <<'EOF'
common:
  log_level: info
cscli:
  hub_branch: master
  output: human
db_config:
  type: sqlite
EOF
}

# derived_in <dir>: derived files still present in a directory (one path per line).
derived_in() {
    for _f in "$1"/.charon-hub-mirror-*; do
        if [ -e "$_f" ]; then
            echo "$_f"
        fi
    done
    return 0
}

# normcalls <file>: the call log with the derived path replaced by the word derived.
normcalls() {
    sed 's#cfg=[^ ]*\.yaml#cfg=derived#' "$1"
}

# prepared <live config> <name>: runs hub_mirror_prepare in a subshell, leaves the derived path in $WORK/<name>.path
prepared() {
    (
        . "$HELPER"
        hub_mirror_prepare "$1" && printf '%s\n' "${CHARON_CSCLI_MIRROR_CONFIG:-}"
    ) >"$WORK/$2.path" 2>/dev/null
    cat "$WORK/$2.path"
}

# ---------------------------------------------------------------------------
echo "# template validation"
(
    . "$HELPER"
    if hub_mirror_template_valid "$TEMPLATE"; then
        echo "valid-default=yes"
    fi
    for t in 'http://x/%s/%s' 'https://x/%s' 'https://x/%s/%s/%s' 'https://x/%s/%d' 'https://x/%s/%s%' \
        'https://x/%s%s%' 'https://x/%S/%s' 'https://x/%%s/%s' 'https://x/%s/%s
' ; do
        if hub_mirror_template_valid "$t"; then
            echo "invalid-accepted"
        fi
    done
) >"$WORK/validate.out" 2>&1
check "default template is valid" "valid-default=yes" "$(grep '^valid-default' "$WORK/validate.out")"
check "invalid templates are refused" "" "$(grep '^invalid-accepted' "$WORK/validate.out")"

# ---------------------------------------------------------------------------
echo "# derived config content"
C1="$WORK/c1"
make_live "$C1"
D1=$(prepared "$C1/config.yaml" c1)
check "prepare exports a readable derived path" "1" "$([ -r "$D1" ] && echo 1)"
cat >"$WORK/c1.expected" <<EOF
common:
  log_level: info
cscli:
$TEMPLATE_LINE
  hub_branch: master
  output: human
db_config:
  type: sqlite
EOF
check_file "template is inserted under cscli, quoted, with the rest kept" "$WORK/c1.expected" "$D1"
check "derived config is mode 0600" "600" "$(stat -c %a "$D1")"
check "live config gets no template" "0" "$(grep -c '__hub_url_template__' "$C1/config.yaml")"

(
    . "$HELPER"
    hub_mirror_prepare "$C1/config.yaml"
    hub_mirror_remove
    hub_mirror_prepare "$C1/config.yaml"
    printf '%s\n' "${CHARON_CSCLI_MIRROR_CONFIG:-}"
) >"$WORK/c1b.path" 2>/dev/null
D1B=$(cat "$WORK/c1b.path")
check_file "deriving again gives identical content (idempotent)" "$D1" "$D1B"
check "one derived file after a repeat" "1" "$(derived_in "$C1" | wc -l | tr -d ' ')"

C2="$WORK/c2"
mkdir -p "$C2"
printf 'common:\n  x: 1\n' >"$C2/config.yaml"
D2=$(prepared "$C2/config.yaml" c2)
printf 'common:\n  x: 1\n\ncscli:\n%s\n  hub_branch: master\n' "$TEMPLATE_LINE" >"$WORK/c2.expected"
check_file "missing cscli section is appended with master pinned" "$WORK/c2.expected" "$D2"

C3="$WORK/c3"
mkdir -p "$C3"
printf 'cscli:\n  hub_branch:\n  __hub_url_template__:\n  output: human\n' >"$C3/config.yaml"
D3=$(prepared "$C3/config.yaml" c3)
printf 'cscli:\n%s\n  hub_branch: master\n  output: human\n' "$TEMPLATE_LINE" >"$WORK/c3.expected"
check_file "empty hub_branch and empty template key are replaced once" "$WORK/c3.expected" "$D3"

C4="$WORK/c4"
mkdir -p "$C4"
printf 'cscli:\n    output: human\n' >"$C4/config.yaml"
D4=$(prepared "$C4/config.yaml" c4)
printf 'cscli:\n    __hub_url_template__: "%s"\n    hub_branch: master\n    output: human\n' "$TEMPLATE" >"$WORK/c4.expected"
check_file "inserted keys follow a 4-space section" "$WORK/c4.expected" "$D4"

C5="$WORK/c5"
mkdir -p "$C5"
printf 'cscli:\n  __hub_url_template__: ""\n  hub_branch: ""\n' >"$C5/config.yaml"
D5=$(prepared "$C5/config.yaml" c5)
printf 'cscli:\n%s\n  hub_branch: master\n' "$TEMPLATE_LINE" >"$WORK/c5.expected"
check_file "quoted empty template and empty pin are unset values (as on the Go side)" "$WORK/c5.expected" "$D5"

# ---------------------------------------------------------------------------
echo "# refusals leave no file and export nothing"
refuse_case() {
    _dir="$WORK/$1"
    mkdir -p "$_dir"
    printf '%b' "$2" >"$_dir/config.yaml"
    (
        . "$HELPER"
        if hub_mirror_prepare "$_dir/config.yaml"; then
            echo "result=accepted"
        else
            echo "result=refused"
        fi
        echo "exported=${CHARON_CSCLI_MIRROR_CONFIG:-}"
    ) >"$_dir/out" 2>&1
    check "$3: refused" "result=refused" "$(grep '^result=' "$_dir/out")"
    check "$3: nothing exported" "exported=" "$(grep '^exported=' "$_dir/out")"
    check "$3: no derived file left" "" "$(derived_in "$_dir")"
}
refuse_case r1 'cscli:\n  __hub_url_template__: "https://example.com/%s/%s"\n' "operator template set"
refuse_case r2 'cscli: {}\n' "inline cscli value"
refuse_case r3 'cscli:\n  output: human\ncscli:\n  hub_branch: master\n' "two cscli sections"

C6="$WORK/c6"
make_live "$C6"
printf 'hub_dir: /custom/hub\n' >"$C6/config.yaml.local"
D6=$(prepared "$C6/config.yaml" c6)
check_file "config.yaml.local is copied beside the derived config" "$C6/config.yaml.local" "$D6.local"
check "copied local override is mode 0600" "600" "$(stat -c %a "$D6.local")"

C7="$WORK/c7"
make_live "$C7"
mkdir "$C7/config.yaml.local"
D7=$(prepared "$C7/config.yaml" c7)
check "unreadable local override fails the prepare" "" "$D7"
check "unreadable local override leaves no derived file" "" "$(derived_in "$C7")"

D8=$(prepared "$WORK/nope/config.yaml" c8)
check "missing live config is refused" "" "$D8"

# ---------------------------------------------------------------------------
echo "# relative path, startup sweep and removal"
C9="$WORK/c9"
make_live "$C9"
(
    cd "$C9" || exit 1
    . "$HELPER"
    hub_mirror_prepare config.yaml
    printf '%s\n' "${CHARON_CSCLI_MIRROR_CONFIG:-}"
) >"$WORK/c9.path" 2>/dev/null
REL=$(cat "$WORK/c9.path")
check "relative live path puts the derived config beside it" "1" "$([ -r "$C9/${REL#./}" ] && echo 1)"

printf 'old\n' >"$C9/.charon-hub-mirror-999.yaml"
printf 'old\n' >"$C9/.charon-hub-mirror-999.yaml.local"
printf 'keep\n' >"$C9/keep.yaml"
(
    . "$HELPER"
    hub_mirror_prepare "$C9/config.yaml"
) >/dev/null 2>&1
check "startup sweep removes a crash leftover" "" "$(derived_in "$C9" | grep 'charon-hub-mirror-999' || true)"
check "startup sweep keeps unrelated files" "1" "$([ -r "$C9/keep.yaml" ] && echo 1)"

(
    . "$HELPER"
    hub_mirror_prepare "$C9/config.yaml"
    hub_mirror_remove
    printf 'after=%s\n' "${CHARON_CSCLI_MIRROR_CONFIG:-unset}"
) >"$WORK/c9r.out" 2>&1
check "remove forgets the derived path" "after=unset" "$(grep '^after=' "$WORK/c9r.out")"
check "remove deletes the derived config and its copy" "" "$(derived_in "$C9")"

(
    . "$HELPER"
    CHARON_CSCLI_MIRROR_CONFIG="$C9/config.yaml"
    hub_mirror_remove
)
check "remove never deletes a file that is not a derived config" "1" "$([ -r "$C9/config.yaml" ] && echo 1)"

# ---------------------------------------------------------------------------
echo "# cscli_hub retry rules"
C10="$WORK/c10"
make_live "$C10"
printf 'hub_dir: /custom/hub\n' >"$C10/config.yaml.local"

# run_case <name> <mode> <mirror first: 1|0> <prepare: 1|0> <budget> <cscli arguments...>
# Runs cscli_hub in its own shell. Results go to $WORK/<name>.res, the call log to $WORK/<name>.calls.
run_case() {
    _name=$1
    _mode=$2
    _first=$3
    _prep=$4
    _budget=$5
    shift 5
    FAKE_MODE=$_mode
    FAKE_LOG="$WORK/$_name.calls"
    export FAKE_MODE FAKE_LOG
    : >"$FAKE_LOG"
    (
        . "$HELPER"
        if [ "$_first" = 1 ]; then
            CHARON_CSCLI_MIRROR_FIRST=1
            export CHARON_CSCLI_MIRROR_FIRST
        fi
        if [ "$_prep" = 1 ]; then
            hub_mirror_prepare "$C10/config.yaml" || echo "prepare-failed"
        fi
        _rc=0
        cscli_hub "$_budget" "$@" >"$WORK/$_name.out" 2>&1 || _rc=$?
        printf 'rc=%s\n' "$_rc"
        printf 'flag=%s\n' "${CHARON_CSCLI_MIRROR_FIRST:-}"
        hub_mirror_remove
        printf 'left=%s\n' "$(derived_in "$C10" | wc -l | tr -d ' ')"
    ) >"$WORK/$_name.res" 2>&1
}
res() {
    grep "^$2=" "$WORK/$1.res" | head -n 1 | sed "s/^$2=//"
}
ncalls() {
    wc -l <"$WORK/$1.calls" | tr -d ' '
}
call() {
    normcalls "$WORK/$1.calls" | sed -n "$2p"
}

run_case live_ok live_ok 0 1 4 hub update
check "live success: exit 0" "0" "$(res live_ok rc)"
check "live success: one call, on the live config" "1" "$(ncalls live_ok)"
check "live success: no mirror mode" "" "$(res live_ok flag)"

run_case live_net live_net 0 1 4 hub update
check "network failure: exit 0 after the retry" "0" "$(res live_net rc)"
check "network failure: two calls" "2" "$(ncalls live_net)"
check "network failure: the retry sees the template and the .local copy" \
    "cfg=derived tpl=1 local=1 args=hub update" "$(call live_net 2)"
check "network failure: mirror mode turned on" "1" "$(res live_net flag)"
check "network failure: derived files removed after the run" "0" "$(res live_net left)"

run_case live_bad live_bad 0 1 4 hub update
check "item error: exit non-zero" "1" "$(res live_bad rc)"
check "item error: not retried" "1" "$(ncalls live_bad)"
check "item error: no mirror mode" "" "$(res live_bad flag)"

run_case live_hang live_hang 0 1 4 hub update
check "hub update timeout: retried and answered" "0" "$(res live_hang rc)"
check "hub update timeout: two calls" "2" "$(ncalls live_hang)"
check "hub update timeout: the retry is on the derived config" \
    "cfg=derived tpl=1 local=1 args=hub update" "$(call live_hang 2)"

run_case hang_item live_hang 0 1 4 collections install crowdsecurity/x
check "item install timeout: not retried" "1" "$(ncalls hang_item)"
check "item install timeout: fails" "1" "$([ "$(res hang_item rc)" != 0 ] && echo 1)"

run_case item_net live_net 0 1 60 collections install crowdsecurity/x
check "item install network failure: live install, derived hub update, derived install" "3" "$(ncalls item_net)"
check "item install retry runs hub update under the derived config first" \
    "cfg=derived tpl=1 local=1 args=hub update" "$(call item_net 2)"
check "item install retry then runs the install under the derived config" \
    "cfg=derived tpl=1 local=1 args=collections install crowdsecurity/x" "$(call item_net 3)"
check "item install retry answered: mirror mode turned on" "1" "$(res item_net flag)"

run_case upgrade_net live_net 0 1 4 hub upgrade
check "hub upgrade network failure: hub update runs before the retry" "3" "$(ncalls upgrade_net)"
check "hub upgrade retry: the pre-update is a hub update" \
    "cfg=derived tpl=1 local=1 args=hub update" "$(call upgrade_net 2)"

run_case both_net both_net 0 1 4 hub update
check "both networks down: exit non-zero" "1" "$([ "$(res both_net rc)" != 0 ] && echo 1)"
check "both networks down: no mirror mode" "" "$(res both_net flag)"

run_case no_derived live_net 0 0 4 hub update
check "no derived config: no retry" "1" "$(ncalls no_derived)"

run_case not_network live_net 0 1 4 hub list
check "non-network command: one attempt, no retry" "1" "$(ncalls not_network)"

run_case mirror_first live_net 1 1 4 hub update
check "mirror first: the first call is the derived config" \
    "cfg=derived tpl=1 local=1 args=hub update" "$(call mirror_first 1)"
check "mirror first: one call when the mirror answers" "1" "$(ncalls mirror_first)"
check "mirror first: mode stays on" "1" "$(res mirror_first flag)"

run_case rescued derived_net 1 1 60 collections install crowdsecurity/x
check "mirror first, rescued by the live config: exit 0" "0" "$(res rescued rc)"
check "mirror first, rescued: derived install, then live hub update and live install" "3" "$(ncalls rescued)"
check "mirror first, rescued: the live hub update runs before the live install" \
    "cfg=live tpl=0 local=0 args=hub update" "$(call rescued 2)"
check "mirror first, rescued: mirror mode turned off" "" "$(res rescued flag)"

run_case clean live_ok 0 1 4 hub update
check "live success leaves no mirror mode behind" "" "$(res clean flag)"

# ---------------------------------------------------------------------------
echo "# install path (configs/crowdsec/install_hub_items.sh)"
INSTALL_SRC="$REPO_DIR/configs/crowdsec/install_hub_items.sh"
# The patterns below are literal source text to find, so the dollar signs must not expand.
# shellcheck disable=SC2016
check "install script bounds each install with cscli_hub 60" "1" \
    "$(grep -c 'cscli_hub 60 "${type}" install "${name}"' "$INSTALL_SRC")"
# shellcheck disable=SC2016
check "install script keeps inspect plain (no mirror, no timeout)" "1" \
    "$(grep -c 'cscli "\$1" inspect "\$2"' "$INSTALL_SRC")"

# An install that hangs is cut at its budget and not retried (no network text in the output).
run_case hang_install_budget live_hang 0 1 2 collections install crowdsecurity/x
check "install hang: cut at its budget, one attempt" "1" "$(ncalls hang_install_budget)"

# run_install <name> <mode> <mirror first 1|0> <prepare 1|0> <helper path>
# Runs a copy of install_hub_items.sh with the helper path pointed at <helper path>.
run_install() {
    _name=$1
    _mode=$2
    _first=$3
    _prep=$4
    _helper=$5
    FAKE_MODE=$_mode
    FAKE_LOG="$WORK/$_name.calls"
    export FAKE_MODE FAKE_LOG
    : >"$FAKE_LOG"
    sed "s#/usr/local/lib/charon/cscli-hub-mirror.sh#$_helper#" "$INSTALL_SRC" >"$WORK/$_name.sh"
    (
        if [ "$_first" = 1 ]; then
            CHARON_CSCLI_MIRROR_FIRST=1
            export CHARON_CSCLI_MIRROR_FIRST
        fi
        if [ "$_prep" = 1 ]; then
            . "$HELPER"
            hub_mirror_prepare "$C10/config.yaml" || echo "prepare-failed"
        fi
        _rc=0
        sh "$WORK/$_name.sh" >"$WORK/$_name.out" 2>&1 || _rc=$?
        printf 'rc=%s\n' "$_rc"
        printf 'flag=%s\n' "${CHARON_CSCLI_MIRROR_FIRST:-}"
        if command -v hub_mirror_remove >/dev/null 2>&1; then
            hub_mirror_remove
        fi
    ) >"$WORK/$_name.res" 2>&1
}

run_install inst_ok live_ok 0 0 "$HELPER"
check "install ok: exit 0" "rc=0" "$(grep '^rc=' "$WORK/inst_ok.res")"
check "install ok: installs run on the live config only" "0" "$(grep -c 'cfg=[^ ]*\.yaml' "$WORK/inst_ok.calls")"

run_install inst_net live_net 0 1 "$HELPER"
check "install network failure: exit 0 (failures are not fatal)" "rc=0" "$(grep '^rc=' "$WORK/inst_net.res")"
check "install network failure: exactly one live install attempt" "1" \
    "$(grep -c 'cfg=live .*install' "$WORK/inst_net.calls")"
check "install network failure: the retry runs hub update first" "1" \
    "$(grep -c 'cfg=[^ ]*\.yaml .*args=hub update' "$WORK/inst_net.calls")"
check "install network failure: mirror mode turned on for later installs" "1" \
    "$(grep -c 'cfg=[^ ]*\.yaml .*args=.*install' "$WORK/inst_net.calls" | awk '{ print ($1 >= 2) ? 1 : 0 }')"

run_install inst_flag live_net 1 1 "$HELPER"
check "install with mirror-first flag: no live install attempt at all" "0" \
    "$(grep -c 'cfg=live .*install' "$WORK/inst_flag.calls")"
check "install with mirror-first flag: installs go to the derived config" "1" \
    "$(grep -c 'cfg=[^ ]*\.yaml .*install' "$WORK/inst_flag.calls" | awk '{ print ($1 >= 1) ? 1 : 0 }')"
check "install with mirror-first flag: exit 0" "rc=0" "$(grep '^rc=' "$WORK/inst_flag.res")"

run_install inst_bad live_bad 0 1 "$HELPER"
check "install item error: not retried on the mirror" "0" "$(grep -c 'cfg=[^ ]*\.yaml' "$WORK/inst_bad.calls")"
check "install item error: reported and the script goes on" "1" \
    "$(grep -c 'Failed to install' "$WORK/inst_bad.out" | awk '{ print ($1 >= 1) ? 1 : 0 }')"

run_install inst_nohelper live_net 0 1 "$WORK/no-such-helper.sh"
check "install without the helper: plain install, no mirror retry" "0" \
    "$(grep -c 'cfg=[^ ]*\.yaml' "$WORK/inst_nohelper.calls")"
check "install without the helper: still exit 0" "rc=0" "$(grep '^rc=' "$WORK/inst_nohelper.res")"

run_install inst_nohelper_ok live_ok 0 0 "$WORK/no-such-helper.sh"
check "install without the helper: installs run on the live config" "1" \
    "$(grep -c 'cfg=live .*install' "$WORK/inst_nohelper_ok.calls" | awk '{ print ($1 >= 1) ? 1 : 0 }')"

# ---------------------------------------------------------------------------
echo "# set -e safety and the stop-aware runner"
(
    set -e
    . "$HELPER"
    FAKE_MODE=live_bad
    FAKE_LOG="$WORK/sete.calls"
    export FAKE_MODE FAKE_LOG
    : >"$FAKE_LOG"
    hub_mirror_prepare "$C10/config.yaml"
    if ! cscli_hub 4 hub update >/dev/null 2>&1; then
        echo "handled"
    fi
    echo "after"
    hub_mirror_remove
) >"$WORK/sete.out" 2>&1
check "set -e: a failed cscli_hub inside if ! does not end the caller" "handled,after" \
    "$(grep -E '^(handled|after)$' "$WORK/sete.out" | tr '\n' ',' | sed 's/,$//')"

(
    set -e
    . "$HELPER"
    . "$HELPER"
    echo "sourced-twice"
) >"$WORK/source2.out" 2>&1
check "set -e: sourcing twice is harmless" "sourced-twice" "$(cat "$WORK/source2.out")"

(
    set -e
    # shellcheck disable=SC2329  # invoked by cscli_hub, not by name
    run_interruptible() {
        echo "custom-runner"
        "$@"
    }
    . "$HELPER"
    FAKE_MODE=live_ok
    FAKE_LOG="$WORK/custom.calls"
    export FAKE_MODE FAKE_LOG
    : >"$FAKE_LOG"
    cscli_hub 4 hub update
) >"$WORK/custom.out" 2>&1
check "an existing run_interruptible is used, not replaced" "1" "$(grep -c '^custom-runner$' "$WORK/custom.out")"

# ---------------------------------------------------------------------------
echo "# syntax of the helper and the entrypoint"
for s in sh dash; do
    if command -v "$s" >/dev/null 2>&1; then
        check "$s -n on the helper" "0" "$("$s" -n "$HELPER" >/dev/null 2>&1; echo $?)"
        check "$s -n on the entrypoint" "0" "$("$s" -n "$ENTRYPOINT" >/dev/null 2>&1; echo $?)"
    fi
done

echo
echo "passed: $PASS, failed: $FAILS"
[ "$FAILS" -eq 0 ]
