package orthrus

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"path"
	"regexp"
	"slices"
	"strings"

	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/util"
	"golang.org/x/time/rate"
)

// AuditLogger is the narrow interface Muzzle depends on for write-path audit
// logging. Satisfied by *services.SecurityService at the call site in
// session.go, where the concrete type is available. Muzzle cannot import
// services directly: services/orthrus_service.go already imports
// "github.com/Wikid82/charon/backend/internal/orthrus", so an orthrus →
// services import would create a cycle.
type AuditLogger interface {
	LogAudit(a *models.SecurityAudit) error
}

// sanitizePath strips newlines and carriage returns from a path string to
// prevent log injection (CWE-117).
func sanitizePath(p string) string {
	p = strings.ReplaceAll(p, "\n", `\n`)
	p = strings.ReplaceAll(p, "\r", `\r`)
	return p
}

// versionPrefixRe strips the Docker API version prefix from a request path,
// e.g. /v1.44/containers/json → /containers/json.
var versionPrefixRe = regexp.MustCompile(`^/v\d+\.\d+`)

// allowedDockerPaths is the set of Docker API paths that are safe to expose to agents.
// Path matching is performed after stripping the version prefix.
var allowedDockerPaths = map[string]struct{}{
	"/_ping":           {},
	"/containers/json": {},
	"/images/json":     {},
	"/info":            {},
	"/version":         {},
	"/events":          {},
	"/volumes":         {},
	"/networks":        {},
	"/system/df":       {},
}

// allowedDockerPatterns covers dynamic-segment paths such as
// /containers/{id}/json, /volumes/{name}, and /networks/{id}.
// Matching uses path.Match after the version prefix has been stripped.
var allowedDockerPatterns = []string{
	"/containers/*/json",
	"/containers/*/logs",
	"/containers/*/stats",
	"/containers/*/top",
	"/volumes/*",
	"/networks/*",
}

// allowedDockerPrefixSuffixPatterns covers /images/*/json and
// /distribution/*/json specifically. These are matched by prefix/suffix
// string comparison rather than path.Match because Go's path.Match "*"
// does not cross "/", and real-world Docker image references are
// frequently namespaced with multiple "/"-separated segments (e.g.
// "ghcr.io/org/repo", "lscr.io/linuxserver/prowlarr"). A path.Match-based
// pattern like "/images/*/json" only matches single-segment names (e.g.
// "nginx") and rejects the overwhelming majority of real images, so it is
// not used here.
//
// Both entries are GET-only like every other allowlist entry: the
// unconditional method check in ServeHTTP runs before any path match, so
// POST/PUT/DELETE to either path is rejected regardless of this list.
// Neither permits a write/mutate operation — /images/create (image pull)
// is deliberately not added.
//
// Traversal segments ("..") cannot be smuggled through the prefix/suffix
// check: ServeHTTP runs path.Clean on stripped before any allowlist check
// (see the comment there), so a path like "/images/../etc/json" is
// normalized to "/etc/json" — which fails the "/images/" prefix — before
// this matching ever runs.
//
// The suffix check ("/json") stays narrow despite allowing multiple path
// segments in the middle: real Docker Engine API endpoints under
// /images/{name}/ that are NOT safe to expose end in a distinct final
// segment rather than literally "json" — /images/{name}/history,
// /images/{name}/get, and /images/{name}/changes all fail the "/json"
// suffix check and remain 403. Only paths whose final segment is exactly
// "json" match, which corresponds to image/distribution inspect — the
// intended read-only operation.
//
// /distribution/*/json causes the remote Docker daemon to make its own
// outbound request to the registry host encoded in the image name —
// read-only here means "no local Docker mutation," not "no outbound
// network activity."
var allowedDockerPrefixSuffixPatterns = []struct {
	prefix string
	suffix string
}{
	{prefix: "/images/", suffix: "/json"},       // image inspect (RepoDigests) — read-only, used by update-checker tools
	{prefix: "/distribution/", suffix: "/json"}, // registry digest check — read-only, used by update-checker tools
}

// allowedWriteExactPaths lists the fixed-path write endpoints permitted when
// a session has negotiated write mode. Only consulted for POST requests, and
// only when Muzzle.writeEnabled is true — see the write-mode gate at the top
// of ServeHTTP. /containers/create additionally requires its body to pass
// validateContainerCreateBody; /images/create takes its parameters via query
// string (fromImage=, tag=) and has no body to validate.
var allowedWriteExactPaths = map[string]struct{}{
	"/containers/create": {},
	"/images/create":     {},
}

// allowedWritePatterns lists the dynamic-segment write endpoints permitted
// when write mode is on: start/stop/restart/rename an existing container,
// or remove one outright. Each entry pairs the exact HTTP method required
// with a path.Match pattern — unlike allowedDockerPrefixSuffixPatterns,
// these patterns operate on Docker-generated container IDs (a single path
// segment, never namespaced), so path.Match's "no cross-slash" behavior is
// the correct, sufficient tool here.
//
// /containers/*/rename takes the new container name via a "?name=" query
// parameter, not a request body (unlike /containers/create), so — like
// start/stop/restart — it needs no body-validation function; the query
// string is not even visible here, since ServeHTTP/Allow match against
// r.URL.Path, not RequestURI. This is Dockhand's standard update pattern:
// rename the old container out of the way (e.g. to "<name>_old") before
// bringing up the new one under the original name.
var allowedWritePatterns = []struct {
	method  string
	pattern string
}{
	{method: http.MethodPost, pattern: "/containers/*/start"},
	{method: http.MethodPost, pattern: "/containers/*/stop"},
	{method: http.MethodPost, pattern: "/containers/*/restart"},
	{method: http.MethodPost, pattern: "/containers/*/rename"},
	{method: http.MethodDelete, pattern: "/containers/*"},
}

// maxContainerCreateBodyBytes bounds the size of a /containers/create
// request body accepted for validation. Generously larger than any
// realistic container-create body (typically a few KB even with a long
// env/label list) — bounds worst-case JSON-parse cost. This constant must
// stay numerically identical to agent/muzzle's copy; the shared test corpus
// (Section 3.3.5 of the write-mode spec) includes a boundary-size case to
// catch drift between the two.
const maxContainerCreateBodyBytes = 64 * 1024

// hostConfigAllowedKeys is the exhaustive allowlist of HostConfig top-level
// keys considered safe for a same-host container recreate. A key NOT in
// this set causes the whole /containers/create request to be rejected —
// this is what gives fail-closed behavior against Docker Engine API fields
// this list's authors don't know about yet, rather than trying to enumerate
// every dangerous key by name. The keys deliberately kept OUT of this set,
// and why, are grouped below rather than left to guesswork:
//
//   - Privileged, CapAdd, Devices, DeviceCgroupRules, DeviceRequests (GPU/device
//     passthrough), SecurityOpt (SELinux/AppArmor/seccomp override), Sysctls,
//     Runtime (arbitrary OCI runtime selection): direct host-escape or
//     isolation-bypass primitives.
//   - Binds, VolumeDriver (a top-level echo of the same local-driver
//     bind-mount-via-volume bypass validateMountsValue's DriverConfig check
//     closes for Mounts): host-filesystem-access primitives. VolumesFrom is
//     NOT in this group — see the value-checked fields below; unlike Binds,
//     it has a narrow legitimate use (containers that share config/media
//     volumes with a companion container) gated behind an explicit
//     per-agent operator allowlist, rather than being blanket-dangerous the
//     way an arbitrary host bind mount is.
//   - PidMode, IpcMode, UTSMode, Cgroup, CgroupParent, GroupAdd: namespace/
//     cgroup-placement fields whose risk depends on what the named
//     namespace, cgroup, or host GID actually grants — not blanket-safe the
//     way a resource *limit* is, so excluded along with the rest of this
//     group rather than guessed at. CgroupnsMode is NOT in this group — see
//     the value-checked fields below; it is a two-valued field with the same
//     "private" (safe, and the actual Docker default since 20.10) vs "host"
//     (shares the host's cgroup namespace) shape as NetworkMode/UsernsMode,
//     not an open-ended namespace/cgroup reference.
//   - Ulimits: NOT excluded as dangerous — see the resource-limit group
//     below. Listed here only because earlier revisions of this comment
//     mis-grouped it as excluded; it is allowed.
//
// Every key actually in the map below is safe because it either (a) only
// limits/throttles a resource the container already has, never grants new
// access; (b) only restricts the container's own view of itself further
// (MaskedPaths, ReadonlyPaths); (c) is cosmetic/metadata with no runtime
// effect (ConsoleSize, Annotations); or (d) operates entirely inside the
// container's own namespaces (Tmpfs, ShmSize — unlike a bind mount, these
// never touch the host filesystem). Grouped by kind, not alphabetically:
//
//	CPU/memory/IO/pid resource limits (Resources, embedded flat into
//	HostConfig's JSON — CpuShares, NanoCpus, Memory, MemorySwap already
//	present above): CpuPeriod, CpuQuota, CpuRealtimePeriod,
//	CpuRealtimeRuntime, CpusetCpus, CpusetMems, BlkioWeight,
//	BlkioWeightDevice, BlkioDeviceReadBps, BlkioDeviceWriteBps,
//	BlkioDeviceReadIOps, BlkioDeviceWriteIOps, KernelMemory,
//	KernelMemoryTCP, MemoryReservation, MemorySwappiness, OomKillDisable,
//	OomScoreAdj, PidsLimit, Ulimits, StorageOpt, ShmSize, CPUCount,
//	CPUPercent, IOMaximumIOps, IOMaximumBandwidth. The last four are
//	Windows-only Resources fields, but the Docker Engine API on Linux
//	daemons still always serializes them (as 0) since they have no
//	omitempty tag — a Linux container recreate that echoes back its full
//	inspected HostConfig will include them regardless of host OS.
//	Container-internal-only mounts: Tmpfs (mirrors the already-allowed
//	Type:"tmpfs" Mounts entries — never a host path).
//	Restriction-only (narrow the container's own view, cannot grant host
//	access): MaskedPaths, ReadonlyPaths.
//	Cosmetic/metadata, no runtime security effect: ConsoleSize, Annotations.
//	Networking convenience, same risk class as the already-allowed
//	ExtraHosts/DnsSearch (hostname/port aliasing, not host filesystem or
//	capability access): Links, PublishAllPorts, DnsOptions.
//	Platform-selection, not a resource limit but still never grants new
//	access: Isolation (Windows-only "default"/"process"/"hyperv" container
//	isolation technology selector — "hyperv" is stronger, VM-based
//	isolation, "process" is comparable to Linux container isolation;
//	neither is a host-escape primitive. Included in this Windows-only
//	group for the same reason as CPUCount etc. above — it can still be
//	present in a Linux daemon's inspect response).
//
// NetworkMode, Mounts, UsernsMode, CgroupnsMode, ContainerIDFile, and
// VolumesFrom additionally receive a value-level check (see
// validateNetworkModeValue, validateMountsValue, validateUsernsModeValue,
// validateCgroupnsModeValue, validateContainerIDFileValue,
// validateVolumesFromValue) beyond simple key presence: each is
// operationally necessary and so cannot be blanket-excluded the way the
// fields above are, but each can still express a host-escape (NetworkMode,
// UsernsMode, CgroupnsMode), host-filesystem (Mounts), host-file-creation
// (ContainerIDFile — the daemon creates a file at this path on the host
// before the container even starts), or opaque-mount-inheritance
// (VolumesFrom) primitive through its value rather than its mere presence.
//
// VolumesFrom is unlike the other five: the muzzle has no way to inspect
// what mounts the *named* container actually has (it may predate Orthrus,
// or have been created outside it entirely), so there is no single value
// shape ("host" vs not, empty vs not) that is inherently safe or unsafe the
// way there is for NetworkMode/UsernsMode/CgroupnsMode/ContainerIDFile.
// Instead, validateVolumesFromValue checks every referenced container
// name/ID against Muzzle.allowedVolumesFromSources — an explicit, per-agent,
// operator-configured allowlist (OrthrusAgent.AllowedVolumesFromSources) —
// and rejects the request if any entry isn't on it. An unconfigured
// (empty/nil) allowlist rejects any non-empty VolumesFrom outright, so this
// field stays fail-closed by default exactly like every other key in this
// map.
var hostConfigAllowedKeys = map[string]struct{}{
	"PortBindings":         {},
	"RestartPolicy":        {},
	"Memory":               {},
	"MemorySwap":           {},
	"NanoCpus":             {},
	"CpuShares":            {},
	"Mounts":               {},
	"Dns":                  {},
	"DnsSearch":            {},
	"DnsOptions":           {},
	"ExtraHosts":           {},
	"LogConfig":            {},
	"AutoRemove":           {},
	"ReadonlyRootfs":       {},
	"Init":                 {},
	"NetworkMode":          {},
	"CapDrop":              {},
	"UsernsMode":           {},
	"ContainerIDFile":      {},
	"CpuPeriod":            {},
	"CpuQuota":             {},
	"CpuRealtimePeriod":    {},
	"CpuRealtimeRuntime":   {},
	"CpusetCpus":           {},
	"CpusetMems":           {},
	"BlkioWeight":          {},
	"BlkioWeightDevice":    {},
	"BlkioDeviceReadBps":   {},
	"BlkioDeviceWriteBps":  {},
	"BlkioDeviceReadIOps":  {},
	"BlkioDeviceWriteIOps": {},
	"KernelMemory":         {},
	"KernelMemoryTCP":      {},
	"MemoryReservation":    {},
	"MemorySwappiness":     {},
	"OomKillDisable":       {},
	"OomScoreAdj":          {},
	"PidsLimit":            {},
	"Ulimits":              {},
	"StorageOpt":           {},
	"ShmSize":              {},
	"Tmpfs":                {},
	"MaskedPaths":          {},
	"ReadonlyPaths":        {},
	"ConsoleSize":          {},
	"Annotations":          {},
	"Links":                {},
	"PublishAllPorts":      {},
	"CgroupnsMode":         {},
	"CPUCount":             {},
	"CPUPercent":           {},
	"IOMaximumIOps":        {},
	"IOMaximumBandwidth":   {},
	"VolumesFrom":          {},
	"Isolation":            {},
}

// mountEntry is the subset of Docker's Mount struct this validator inspects.
type mountEntry struct {
	Type          string          `json:"Type"`
	VolumeOptions json.RawMessage `json:"VolumeOptions"`
}

// validateNetworkModeValue rejects "host" networking and container-mode
// networking ("container:<id>", which grants access to another container's
// network namespace) — the two NetworkMode values that carry the same class
// of host-escape risk as the blanket-excluded HostConfig keys. Every other
// value (a bridge network name, "bridge", "none", "default") is accepted,
// since NetworkMode is a normal, operationally required field for any
// container recreate that isn't on the default bridge network.
func validateNetworkModeValue(raw json.RawMessage) bool {
	var mode string
	if err := json.Unmarshal(raw, &mode); err != nil {
		return false
	}
	if mode == "host" {
		return false
	}
	if strings.HasPrefix(mode, "container:") {
		return false
	}
	return true
}

// validateUsernsModeValue rejects only the value "host", which opts a
// container out of Docker's user-namespace remapping and thereby carries the
// same class of host-escape risk as the blanket-excluded HostConfig keys.
// Every other value — the empty string (inherit the daemon's userns-remap
// configuration, the common case for a same-host recreate) or any other
// named mode — is accepted, mirroring validateNetworkModeValue's shape.
func validateUsernsModeValue(raw json.RawMessage) bool {
	var mode string
	if err := json.Unmarshal(raw, &mode); err != nil {
		return false
	}
	return mode != "host"
}

// validateCgroupnsModeValue rejects only the value "host", which shares the
// container's cgroup namespace with the host's — historically relevant to
// cgroup v1 release_agent exploits and general host resource/process-hierarchy
// information disclosure. Every other value, including "private" (Docker's
// default since 20.10) and the empty string (inherit the daemon's default),
// is accepted, mirroring validateUsernsModeValue's shape.
func validateCgroupnsModeValue(raw json.RawMessage) bool {
	var mode string
	if err := json.Unmarshal(raw, &mode); err != nil {
		return false
	}
	return mode != "host"
}

// validateContainerIDFileValue accepts only the empty string. A non-empty
// ContainerIDFile has the Docker daemon create a new file, containing the
// new container's ID, at an operator-chosen path on the HOST filesystem
// (opened O_EXCL, so it cannot clobber an existing file, but it can still
// create one anywhere the daemon's own filesystem permissions allow) —
// before the container itself even starts, so none of the container's own
// isolation applies. That is a real host-filesystem-write primitive, unlike
// every other field in hostConfigAllowedKeys, so it gets the narrowest
// possible value-level check rather than a blanket allow: the empty string
// (the overwhelming common case — the field is rarely set explicitly, and a
// same-host recreate of such a container legitimately echoes it back empty)
// is accepted, and every non-empty value is rejected.
func validateContainerIDFileValue(raw json.RawMessage) bool {
	var cidFile string
	if err := json.Unmarshal(raw, &cidFile); err != nil {
		return false
	}
	return cidFile == ""
}

// validateVolumesFromValue accepts a VolumesFrom value only if every
// referenced container name/ID is present in allowedSources — the
// per-agent, operator-configured allowlist (Muzzle.allowedVolumesFromSources,
// sourced from OrthrusAgent.AllowedVolumesFromSources). A nil/empty
// allowedSources rejects any non-empty VolumesFrom entry, matching this
// muzzle's fail-closed default. Each entry may carry a Docker
// ":ro"/":rw"/":Z" mode suffix (e.g. "other-container:ro"); only the
// container name/ID portion before the first colon is compared, since the
// mode suffix affects mount permissions, not which container's mounts are
// inherited.
func validateVolumesFromValue(raw json.RawMessage, allowedSources []string) bool {
	var sources []string
	if err := json.Unmarshal(raw, &sources); err != nil {
		return false
	}
	for _, source := range sources {
		name, _, _ := strings.Cut(source, ":")
		if !slices.Contains(allowedSources, name) {
			return false
		}
	}
	return true
}

// validateMountsValue rejects any bind-type mount and any mount entry whose
// VolumeOptions carries a DriverConfig key at all, regardless of Type.
//
// The DriverConfig check closes a documented bypass of the Type check
// alone: Docker's default "local" volume driver accepts
// {"type":"none","device":"/host/path","o":"bind"} inside
// VolumeOptions.DriverConfig.Options, which makes a Type:"volume" mount
// behave exactly like an arbitrary host bind mount — fully equivalent in
// effect to the Type:"bind" mount rejected below, but reached through a
// field one level deeper. No attempt is made to selectively allow safe
// DriverConfig.Options key/value pairs: any DriverConfig presence at all is
// rejected, matching this validator's hard-reject-over-silent-strip
// philosophy.
func validateMountsValue(raw json.RawMessage) bool {
	var mounts []mountEntry
	if err := json.Unmarshal(raw, &mounts); err != nil {
		return false
	}
	for _, mnt := range mounts {
		if mnt.Type != "volume" && mnt.Type != "tmpfs" {
			// Rejects "bind" and any unrecognized/future mount type. The
			// legacy HostConfig.Binds field is rejected separately, simply
			// by not appearing in hostConfigAllowedKeys.
			return false
		}
		if len(mnt.VolumeOptions) == 0 {
			continue
		}
		var volumeOptions map[string]json.RawMessage
		if err := json.Unmarshal(mnt.VolumeOptions, &volumeOptions); err != nil {
			return false
		}
		if _, hasDriverConfig := volumeOptions["DriverConfig"]; hasDriverConfig {
			return false
		}
	}
	return true
}

// validateContainerCreateBody reads, size-caps, and validates a
// /containers/create request body against hostConfigAllowedKeys plus the
// NetworkMode/Mounts value-level checks, then re-buffers r.Body so the
// downstream reverse proxy can still forward the original bytes intact —
// reading r.Body consumes it exactly once, so it must be replaced with a
// fresh reader over the same bytes before this function returns, regardless
// of outcome.
//
// Returns (true, "") if the body is safe to forward. Returns (false, reason)
// if the request should be rejected, with reason suitable for both the audit
// log and the HTTP error response.
//
// A method on *Muzzle (rather than a free function, like every other
// validator in this file) solely because validateVolumesFromValue needs
// m.allowedVolumesFromSources, the one per-agent operator-configured value
// this validation pass depends on.
func (m *Muzzle) validateContainerCreateBody(r *http.Request) (ok bool, reason string) {
	if r.Body == nil {
		return true, ""
	}

	limited := io.LimitReader(r.Body, maxContainerCreateBodyBytes+1)
	bodyBytes, err := io.ReadAll(limited)
	_ = r.Body.Close()
	if err != nil {
		r.Body = http.NoBody
		return false, "malformed request body"
	}
	if len(bodyBytes) > maxContainerCreateBodyBytes {
		// Re-buffer the (oversized, rejected) bytes anyway: forwarding never
		// happens on this path, but leaving r.Body in a consumed state would
		// be surprising for any future caller of this function.
		r.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		r.ContentLength = int64(len(bodyBytes))
		return false, "request body too large"
	}

	// Re-buffer before any further validation so every return path below —
	// success or rejection — leaves r.Body forwarding-ready.
	r.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	r.ContentLength = int64(len(bodyBytes))

	if len(bodyBytes) == 0 {
		return true, ""
	}

	var top map[string]json.RawMessage
	if err := json.Unmarshal(bodyBytes, &top); err != nil {
		return false, "malformed request body"
	}

	hostConfigRaw, hasHostConfig := top["HostConfig"]
	if !hasHostConfig {
		return true, ""
	}

	var hostConfig map[string]json.RawMessage
	if err := json.Unmarshal(hostConfigRaw, &hostConfig); err != nil {
		return false, "malformed request body"
	}

	for key, rawValue := range hostConfig {
		if _, allowed := hostConfigAllowedKeys[key]; !allowed {
			return false, "disallowed HostConfig field: " + key
		}
		switch key {
		case "NetworkMode":
			if !validateNetworkModeValue(rawValue) {
				return false, "disallowed HostConfig field: NetworkMode"
			}
		case "Mounts":
			if !validateMountsValue(rawValue) {
				return false, "disallowed HostConfig field: Mounts"
			}
		case "UsernsMode":
			if !validateUsernsModeValue(rawValue) {
				return false, "disallowed HostConfig field: UsernsMode"
			}
		case "ContainerIDFile":
			if !validateContainerIDFileValue(rawValue) {
				return false, "disallowed HostConfig field: ContainerIDFile"
			}
		case "CgroupnsMode":
			if !validateCgroupnsModeValue(rawValue) {
				return false, "disallowed HostConfig field: CgroupnsMode"
			}
		case "VolumesFrom":
			if !validateVolumesFromValue(rawValue, m.allowedVolumesFromSources) {
				return false, "disallowed HostConfig field: VolumesFrom"
			}
		}
	}

	return true, ""
}

// writeAuditDetails marshals a small flat field set into the JSON string
// stored in a SecurityAudit's Details column. Marshal failure (unreachable
// for the string-only maps this is called with) degrades to an empty JSON
// object rather than losing the audit entry entirely.
func writeAuditDetails(fields map[string]string) string {
	b, err := json.Marshal(fields)
	if err != nil {
		return "{}"
	}
	return string(b)
}

// logAudit is a nil-safe wrapper around m.auditLogger.LogAudit. auditLogger
// is nil for read-only sessions and in most tests — a no-op in that case,
// not an error, since read-only traffic was never audited before this
// feature and isn't in scope for it now (see Section 3.3.7 of the
// write-mode spec).
func (m *Muzzle) logAudit(action, details string) {
	if m.auditLogger == nil {
		return
	}
	// Actor identifies the agent, not a Charon user — there is no
	// authenticated operator in this request path, only a third-party tool
	// on the other end of the tunnel. agentUUID (not agent name) is used
	// here because it's what NewMuzzle actually receives; ResourceUUID
	// carries the same value for querying, and a UUID is a more stable
	// identifier for an audit trail than a renamable display name anyway.
	_ = m.auditLogger.LogAudit(&models.SecurityAudit{
		Actor:         "orthrus-agent:" + m.agentUUID,
		Action:        action,
		EventCategory: "orthrus_write",
		ResourceUUID:  m.agentUUID,
		Details:       details,
	})
}

func (m *Muzzle) auditAllowed(r *http.Request) {
	m.logAudit("orthrus_write_allowed", writeAuditDetails(map[string]string{
		"method": r.Method,
		"path":   sanitizePath(r.URL.Path),
	}))
}

func (m *Muzzle) auditBlocked(r *http.Request, reason string) {
	m.logAudit("orthrus_write_blocked", writeAuditDetails(map[string]string{
		"method": r.Method,
		"path":   sanitizePath(r.URL.Path),
		"reason": reason,
	}))
}

func (m *Muzzle) auditRateLimited(r *http.Request) {
	m.logAudit("orthrus_write_rate_limited", writeAuditDetails(map[string]string{
		"method": r.Method,
		"path":   sanitizePath(r.URL.Path),
	}))
}

// Muzzle is an http.Handler wrapper that restricts Docker socket access
// to a curated allowlist of read-only, non-destructive endpoints, plus an
// optional narrow set of write endpoints when writeEnabled is true.
type Muzzle struct {
	next http.Handler
	// writeEnabled is fixed at construction time (one Muzzle per AgentSession,
	// per external-proxy start) — never re-read from the DB per-request. See
	// NewMuzzle doc comment for why.
	writeEnabled bool
	// writeLimiter bounds write-request throughput; nil unless writeEnabled.
	writeLimiter *rate.Limiter
	// auditLogger records every write attempt (allowed or blocked); nil is
	// tolerated (no-op) so tests and read-only sessions don't need one.
	auditLogger AuditLogger
	// agentUUID identifies the session this Muzzle guards, for audit entries.
	agentUUID string
	// allowedVolumesFromSources is the operator-configured allowlist of
	// container names/IDs this session's agent may reference via
	// HostConfig.VolumesFrom (see validateVolumesFromValue). Fixed at
	// construction time like writeEnabled, for the same reconnect-to-apply
	// reason.
	allowedVolumesFromSources []string
}

// NewMuzzle wraps handler with the Docker socket allowlist filter.
//
// writeEnabled, writeLimiter, auditLogger, agentUUID, and
// allowedVolumesFromSources govern the optional write-endpoint allowlist
// (see Section 3.3.3 of the Orthrus write-mode spec). writeEnabled is
// captured once, at construction time, and never re-checked against the
// database on a per-request basis — StartExternalProxy constructs a new
// Muzzle per AgentSession using the value negotiated at connect time, so
// toggling the DB flag only takes effect on the agent's next reconnect.
// Re-reading it per-request would both reintroduce a TOCTOU-like
// inconsistency with that reconnect-to-apply guarantee and add a DB
// round-trip to the hot proxy path. allowedVolumesFromSources is negotiated
// and fixed the same way.
func NewMuzzle(next http.Handler, writeEnabled bool, writeLimiter *rate.Limiter, auditLogger AuditLogger, agentUUID string, allowedVolumesFromSources []string) *Muzzle {
	return &Muzzle{
		next:                      next,
		allowedVolumesFromSources: allowedVolumesFromSources,
		writeEnabled:              writeEnabled,
		writeLimiter:              writeLimiter,
		auditLogger:               auditLogger,
		agentUUID:                 agentUUID,
	}
}

// normalizeDockerPath strips a Docker API version prefix (e.g. "/v1.47")
// using versionPrefixRe, THEN runs path.Clean — in that order. Stripping
// first means the version-prefix match runs against the raw, uncleaned
// path, so a traversal-disguised prefix (e.g. "/foo/../v1.44/...") is not
// mistaken for a real one: versionPrefixRe is anchored to the start of the
// string as given, before ".." segments have been resolved away. Normalize
// away any "." or ".." segments only after that check so that traversal-style
// paths such as /containers/../json cannot match patterns like
// /containers/*/json. path.Clean always returns a rooted result when given a
// rooted input; the explicit "/" prefix guards against an empty stripped
// value.
//
// agent/muzzle/muzzle.go has an identically-named helper that must apply
// versionPrefixRe (same regex source) and path.Clean in this exact order —
// see that file's doc comment and scripts/ci/check_muzzle_allowlist_parity.go,
// which structurally compares the two files' versionPrefixRe declarations.
func normalizeDockerPath(rawPath string) string {
	stripped := versionPrefixRe.ReplaceAllString(rawPath, "")
	return path.Clean("/" + strings.TrimLeft(stripped, "/"))
}

// ServeHTTP implements http.Handler. Only GET requests to allowlisted paths
// are forwarded; HEAD is also permitted for /_ping (Docker client health checks).
// All other methods or paths receive 403 Forbidden.
func (m *Muzzle) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	stripped := normalizeDockerPath(r.URL.Path)

	// HEAD /_ping is permitted alongside GET for Docker client health checks.
	if r.Method == http.MethodHead && stripped == "/_ping" {
		m.next.ServeHTTP(w, r)
		return
	}

	// Write-endpoint handling must run before the unconditional GET-only
	// check below, since POST/DELETE requests would otherwise be rejected
	// before ever reaching the write allowlist. Only consulted when this
	// session negotiated write mode (m.writeEnabled, fixed at construction
	// time — see NewMuzzle) — every other session's POST/DELETE traffic
	// falls straight through, unaffected, to the same 403 it always got.
	if m.writeEnabled && (r.Method == http.MethodPost || r.Method == http.MethodDelete) {
		if m.tryServeWrite(w, r, stripped) {
			return
		}
		// Not on the write allowlist even with write mode on — fall through
		// to the same "Forbidden" response every other disallowed request gets.
	}

	if r.Method != http.MethodGet {
		logger.Log().WithField("method", util.SanitizeForLog(r.Method)).WithField("path", sanitizePath(r.URL.Path)).
			Warn("orthrus: muzzle blocked non-GET Docker request")
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	if _, ok := allowedDockerPaths[stripped]; ok {
		m.next.ServeHTTP(w, r)
		return
	}

	// Check dynamic path patterns for container/volume/network inspection.
	// stripped is already rooted and cleaned; use it directly.
	for _, pat := range allowedDockerPatterns {
		if matched, err := path.Match(pat, stripped); err == nil && matched {
			m.next.ServeHTTP(w, r)
			return
		}
	}

	// Check image/distribution inspect paths, which may contain namespaced
	// references with multiple "/"-separated segments (see doc comment on
	// allowedDockerPrefixSuffixPatterns for why path.Match is not used here).
	for _, ps := range allowedDockerPrefixSuffixPatterns {
		if strings.HasPrefix(stripped, ps.prefix) && strings.HasSuffix(stripped, ps.suffix) {
			m.next.ServeHTTP(w, r)
			return
		}
	}

	logger.Log().WithField("method", util.SanitizeForLog(r.Method)).WithField("path", sanitizePath(r.URL.Path)).
		Warn("orthrus: muzzle blocked disallowed Docker path")
	http.Error(w, "Forbidden", http.StatusForbidden)
}

// tryServeWrite handles a POST/DELETE request when this session has write
// mode enabled. Returns true if it fully handled the request — forwarded,
// rate-limited, or rejected with a write-specific reason and audit entry —
// false if the method/path combination isn't on the write allowlist at all,
// in which case the caller (ServeHTTP) falls through to the same generic
// 403 every other disallowed request receives.
func (m *Muzzle) tryServeWrite(w http.ResponseWriter, r *http.Request, stripped string) bool {
	if r.Method == http.MethodPost {
		if _, ok := allowedWriteExactPaths[stripped]; ok {
			if stripped == "/containers/create" {
				if valid, reason := m.validateContainerCreateBody(r); !valid {
					m.auditBlocked(r, reason)
					http.Error(w, "Forbidden: "+reason, http.StatusForbidden)
					return true
				}
			}
			return m.forwardWrite(w, r)
		}
	}

	for _, p := range allowedWritePatterns {
		if r.Method != p.method {
			continue
		}
		if matched, err := path.Match(p.pattern, stripped); err == nil && matched {
			return m.forwardWrite(w, r)
		}
	}

	return false
}

// forwardWrite applies the write-path rate limiter, audits the outcome, and
// forwards the request on success. Always returns true (the caller uses the
// return value only to distinguish "on the write allowlist" from "not"; a
// rate-limited request is still a request tryServeWrite fully handled).
func (m *Muzzle) forwardWrite(w http.ResponseWriter, r *http.Request) bool {
	if m.writeLimiter != nil && !m.writeLimiter.Allow() {
		m.auditRateLimited(r)
		http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
		return true
	}
	m.auditAllowed(r)
	m.next.ServeHTTP(w, r)
	return true
}
