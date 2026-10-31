# shellcheck shell=sh
# CrowdSec hub mirror retry for cscli. POSIX sh, sourced by .docker/docker-entrypoint.sh and by
# configs/crowdsec/install_hub_items.sh. The image copies it to /usr/local/lib/charon/cscli-hub-mirror.sh.
#
# cscli reads one hub URL template from its config (the hidden key __hub_url_template__). When the hub CDN
# is unreachable, a retry runs cscli with a derived copy of the config whose template points at the raw
# GitHub mirror. The live config is never written.
#
# The rules follow backend/internal/crowdsec/hub_mirror.go (template checks, refusal of a template the
# operator already set, the retry classifier). Keep the two in step.
#
# Internal state, set by this helper and not meant for operators:
#   CHARON_CSCLI_MIRROR_CONFIG  path of the derived config while it exists
#   CHARON_CSCLI_MIRROR_FIRST   "1" once a mirror retry answered: later cscli_hub calls go mirror-first

_HUB_MIRROR_TEMPLATE='https://raw.githubusercontent.com/crowdsecurity/hub/%s/%s'
_HUB_MIRROR_KEY='__hub_url_template__'
_HUB_MIRROR_PREFIX='.charon-hub-mirror-'

# Plain foreground fallback for callers without the entrypoint's stop-aware runner (install_hub_items.sh).
if ! command -v run_interruptible >/dev/null 2>&1; then
    run_interruptible() {
        "$@"
    }
fi

# hub_mirror_template_valid <template>
# Succeeds for an https URL with exactly two %s verbs, no other percent sign and no control characters.
hub_mirror_template_valid() {
    case "$1" in
        https://*%s*%s*) ;;
        *) return 1 ;;
    esac
    _pct=$(printf '%s' "$1" | tr -cd '%' | wc -c | tr -d ' ')
    if [ "$_pct" != 2 ]; then
        return 1
    fi
    case "$1" in
        *[[:cntrl:]]*) return 1 ;;
    esac
    return 0
}

# _hub_mirror_match <ERE> <line>
_hub_mirror_match() {
    printf '%s\n' "$2" | grep -Eq "$1"
}

# _hub_mirror_child_indent <config>
# Prints the indentation of the first child of the cscli section, so an inserted key matches the file.
_hub_mirror_child_indent() {
    _ind=$(awk '
        /^cscli:[[:space:]]*$/ { incscli = 1; next }
        incscli && /^[^[:space:]#]/ { exit }
        incscli && /^[[:space:]]+[^[:space:]#]/ { match($0, /^[[:space:]]+/); print substr($0, 1, RLENGTH); exit }
    ' "$1" 2>/dev/null) || _ind=""
    printf '%s' "${_ind:-  }"
}

# Empty YAML values, the same ones the Go side treats as unset: no value, "", '', ~ and null (optionally followed
# by a comment).
_HUB_MIRROR_EMPTY_VALUE='(""|'"''"'|~|null)?[[:space:]]*(#.*)?$'

# _hub_mirror_rewrite <live config> <template>
# Writes the live config to stdout with the template set in its cscli section. An empty hub_branch is set to
# master, and an empty template key is replaced. Fails (writes nothing usable) when the config already sets a
# non-empty template, has an inline cscli value such as `cscli: {}`, or has more than one cscli section.
_hub_mirror_rewrite() {
    _src=$1
    _tpl=$2
    _indent=$(_hub_mirror_child_indent "$_src")
    _has_branch=0
    while IFS= read -r _line || [ -n "$_line" ]; do
        case "$_line" in
            *hub_branch:*)
                if ! _hub_mirror_match "^[[:space:]]+hub_branch:[[:space:]]*${_HUB_MIRROR_EMPTY_VALUE}" "$_line"; then
                    _has_branch=1
                fi
                ;;
        esac
    done <"$_src"
    _sections=0
    while IFS= read -r _line || [ -n "$_line" ]; do
        case "$_line" in
            cscli:*)
                if ! _hub_mirror_match '^cscli:[[:space:]]*$' "$_line"; then
                    return 1
                fi
                _sections=$((_sections + 1))
                if [ "$_sections" -gt 1 ]; then
                    return 1
                fi
                printf '%s\n' "$_line"
                printf '%s%s: "%s"\n' "$_indent" "$_HUB_MIRROR_KEY" "$_tpl"
                if [ "$_has_branch" -eq 0 ]; then
                    printf '%shub_branch: master\n' "$_indent"
                fi
                ;;
            *"$_HUB_MIRROR_KEY"*)
                # An empty value is dropped (the line above replaces it); any other value means the operator set a hub.
                if ! _hub_mirror_match "^[[:space:]]*${_HUB_MIRROR_KEY}:[[:space:]]*${_HUB_MIRROR_EMPTY_VALUE}" "$_line"; then
                    return 1
                fi
                ;;
            *hub_branch:*)
                # An empty pin is dropped (master is written with the template); a set pin is kept.
                if _hub_mirror_match "^[[:space:]]+hub_branch:[[:space:]]*${_HUB_MIRROR_EMPTY_VALUE}" "$_line"; then
                    continue
                fi
                printf '%s\n' "$_line"
                ;;
            *)
                printf '%s\n' "$_line"
                ;;
        esac
    done <"$_src"
    if [ "$_sections" -eq 0 ]; then
        printf '\ncscli:\n%s%s: "%s"\n' "$_indent" "$_HUB_MIRROR_KEY" "$_tpl"
        if [ "$_has_branch" -eq 0 ]; then
            printf '%shub_branch: master\n' "$_indent"
        fi
    fi
    return 0
}

# _hub_mirror_sweep <dir>
# Removes derived configs a previous run left behind. Other files are never touched.
_hub_mirror_sweep() {
    for _leftover in "$1/$_HUB_MIRROR_PREFIX"*.yaml "$1/$_HUB_MIRROR_PREFIX"*.yaml.local; do
        if [ -e "$_leftover" ]; then
            rm -f "$_leftover"
        fi
    done
    return 0
}

# hub_mirror_prepare <live config path>
# Writes the derived config beside the live one (mode 0600, the live settings with the mirror template), copies
# config.yaml.local next to it when present (CrowdSec merges that file by name), and exports
# CHARON_CSCLI_MIRROR_CONFIG. On failure nothing is left behind and nothing is exported, so cscli_hub does not
# retry. Relative paths keep resolving because the copy sits in the same directory.
hub_mirror_prepare() {
    _live=$1
    if ! hub_mirror_template_valid "$_HUB_MIRROR_TEMPLATE"; then
        return 1
    fi
    if [ ! -r "$_live" ]; then
        return 1
    fi
    _dir=$(dirname "$_live")
    _hub_mirror_sweep "$_dir"
    _derived="$_dir/$_HUB_MIRROR_PREFIX$$.yaml"
    if ! (umask 077 && set -C && _hub_mirror_rewrite "$_live" "$_HUB_MIRROR_TEMPLATE" >"$_derived"); then
        rm -f "$_derived"
        return 1
    fi
    if [ -e "$_live.local" ]; then
        if ! (umask 077 && cp "$_live.local" "$_derived.local"); then
            rm -f "$_derived" "$_derived.local"
            return 1
        fi
    fi
    CHARON_CSCLI_MIRROR_CONFIG=$_derived
    export CHARON_CSCLI_MIRROR_CONFIG
    return 0
}

# hub_mirror_remove
# Deletes the derived config and its local copy, then forgets the path. Only files named like a derived config
# are deleted, whatever the variable holds.
hub_mirror_remove() {
    _derived="${CHARON_CSCLI_MIRROR_CONFIG:-}"
    if [ -n "$_derived" ]; then
        case "$(basename "$_derived")" in
            "$_HUB_MIRROR_PREFIX"*.yaml)
                rm -f "$_derived" "$_derived.local"
                ;;
        esac
    fi
    unset CHARON_CSCLI_MIRROR_CONFIG
    return 0
}

# _hub_mirror_available: a derived config was prepared and is still there.
_hub_mirror_available() {
    [ -n "${CHARON_CSCLI_MIRROR_CONFIG:-}" ] && [ -r "$CHARON_CSCLI_MIRROR_CONFIG" ]
}

# _hub_left <deadline epoch seconds>: seconds left before the deadline (may be zero or negative).
_hub_left() {
    printf '%s' $(($1 - $(date +%s)))
}

# _hub_network_failure <log>: the output names a failed network request.
_hub_network_failure() {
    grep -Eqi 'dial tcp|no such host|network is unreachable|connection refused|i/o timeout|unable to send request|unable to retrieve latest crowdsec version|context deadline exceeded|client\.timeout|tls handshake timeout|lookup [^[:space:]]+ on [^[:space:]]' "$1"
}

# _hub_retry_wanted <kind> <status> <log>
# hub commands retry on a network failure or when timeout ended the attempt. Item installs and upgrades retry on
# a network failure only, so a healthy slow install is never cut short.
_hub_retry_wanted() {
    case "$2" in
        124|143)
            # 124 is GNU timeout's status, 143 is busybox timeout's.
            if [ "$1" = hub ]; then
                return 0
            fi
            ;;
    esac
    _hub_network_failure "$3"
}

# _hub_attempt <seconds> <config or empty> <log> <cscli arguments...>
# Runs one cscli command under timeout with its output in the log, prints the log, and returns the status.
_hub_attempt() {
    _secs=$1
    _cfg=$2
    _log=$3
    shift 3
    if [ "$_secs" -lt 1 ]; then
        # timeout 0 would mean no limit at all, so an attempt with no time left is refused instead.
        : >"$_log"
        return 124
    fi
    if [ -n "$_cfg" ]; then
        set -- -c "$_cfg" "$@"
    fi
    _status=0
    run_interruptible timeout "$_secs" cscli "$@" >"$_log" 2>&1 || _status=$?
    cat "$_log"
    return "$_status"
}

# _hub_retry <deadline> <config or empty> <pre: 1|0> <log> <cscli arguments...>
# Optionally runs `hub update` under the retry config first, then the command, both within the deadline.
_hub_retry() {
    _deadline=$1
    _cfg=$2
    _pre=$3
    _log=$4
    shift 4
    if [ "$_pre" -eq 1 ]; then
        _hub_attempt "$(_hub_left "$_deadline")" "$_cfg" "$_log" hub update || return $?
    fi
    _hub_attempt "$(_hub_left "$_deadline")" "$_cfg" "$_log" "$@"
}

# cscli_hub <budget seconds> <cscli arguments...>
# Runs a cscli command with the retry rules above, all within the budget:
#   hub update, hub upgrade   attempt 1 gets half the budget; the retry gets the rest
#   <type> install|upgrade    attempt 1 gets the whole budget; the retry gets the rest
#   anything else             one attempt, no retry
# The retry uses the derived mirror config when one was prepared. When the mirror is the first choice (after an
# earlier mirror win), the live config is the retry. Before a retry of an install, an upgrade or `hub upgrade`,
# `hub update` runs under the retry config so the index and the items come from one source.
# A mirror retry that answers sets CHARON_CSCLI_MIRROR_FIRST=1; a live retry that answers clears it.
# Returns the status of the last attempt and prints every attempt's output.
cscli_hub() {
    _budget=$1
    shift
    _deadline=$(($(date +%s) + _budget))
    _kind=none
    _first=$_budget
    if [ "${1:-}" = hub ]; then
        case "${2:-}" in
            update|upgrade)
                _kind=hub
                _first=$((_budget / 2))
                ;;
        esac
    else
        case "${2:-}" in
            install|upgrade) _kind=item ;;
        esac
    fi
    if [ "$_first" -lt 1 ]; then
        _first=1
    fi

    _retry_ok=0
    _first_cfg=""
    _retry_cfg=""
    if _hub_mirror_available; then
        _retry_ok=1
        if [ "${CHARON_CSCLI_MIRROR_FIRST:-}" = 1 ]; then
            _first_cfg=$CHARON_CSCLI_MIRROR_CONFIG
        else
            _retry_cfg=$CHARON_CSCLI_MIRROR_CONFIG
        fi
    fi

    _log=$(mktemp "${TMPDIR:-/tmp}/charon-cscli-hub.XXXXXX") || return 1
    _rc=0
    _hub_attempt "$_first" "$_first_cfg" "$_log" "$@" || _rc=$?
    if [ "$_rc" -ne 0 ] && [ "$_kind" != none ] && [ "$_retry_ok" -eq 1 ] &&
        _hub_retry_wanted "$_kind" "$_rc" "$_log" && [ "$(_hub_left "$_deadline")" -gt 0 ]; then
        _pre=0
        if [ "$_kind" = item ] || [ "${2:-}" = upgrade ]; then
            _pre=1
        fi
        if [ -n "$_retry_cfg" ]; then
            echo "cscli $* did not complete on the CDN; retrying through the hub mirror."
        else
            echo "cscli $* did not complete; retrying on the default hub source."
        fi
        _retry_rc=0
        _hub_retry "$_deadline" "$_retry_cfg" "$_pre" "$_log" "$@" || _retry_rc=$?
        _rc=$_retry_rc
        if [ "$_rc" -eq 0 ]; then
            if [ -n "$_retry_cfg" ]; then
                export CHARON_CSCLI_MIRROR_FIRST=1
            else
                unset CHARON_CSCLI_MIRROR_FIRST
            fi
        fi
    fi
    rm -f "$_log"
    return "$_rc"
}
