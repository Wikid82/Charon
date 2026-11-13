package services

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/netip"
	"net/url"
	"strings"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/orthrus"
	"github.com/Wikid82/charon/backend/internal/version"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

const (
	authKeyPrefix  = "ch_orthrus_"
	authKeyRandLen = 32 // bytes of random hex

	// maxExternalProxyBindLen bounds the stored listen setting.
	maxExternalProxyBindLen = 64
)

// ErrInvalidExternalProxyBind is returned when a listen setting is neither
// automatic, all interfaces, an address, nor a subnet. The handler maps it to a
// fixed operator-facing message and error code.
var ErrInvalidExternalProxyBind = errors.New("external proxy bind is invalid")

// agentDisconnector closes a live agent session. *orthrus.OrthrusServer satisfies
// it; tests substitute a recorder.
type agentDisconnector interface {
	DisconnectAgent(agentUUID string) error
}

// OrthrusService provides agent lifecycle management on top of OrthrusServer.
type OrthrusService struct {
	db     *gorm.DB
	server agentDisconnector
}

// NewOrthrusService creates an OrthrusService. A nil server leaves session
// disconnection disabled (no sessions can exist without a server).
func NewOrthrusService(db *gorm.DB, server *orthrus.OrthrusServer) *OrthrusService {
	svc := &OrthrusService{db: db}
	if server != nil {
		svc.server = server
	}
	return svc
}

// List returns all registered OrthrusAgent records.
func (s *OrthrusService) List() ([]models.OrthrusAgent, error) {
	var agents []models.OrthrusAgent
	if err := s.db.Find(&agents).Error; err != nil {
		return nil, fmt.Errorf("orthrus: list agents: %w", err)
	}
	return agents, nil
}

// newAuthKey generates a random agent key and its bcrypt hash. The plaintext is
// returned once to the caller and never stored.
func newAuthKey() (plainKey string, hash []byte, err error) {
	rawBytes := make([]byte, authKeyRandLen)
	if _, err = rand.Read(rawBytes); err != nil {
		return "", nil, fmt.Errorf("orthrus: generate auth key: %w", err)
	}
	plainKey = authKeyPrefix + hex.EncodeToString(rawBytes)

	hash, err = bcrypt.GenerateFromPassword(orthrus.BcryptInput(plainKey), orthrus.BcryptCost)
	if err != nil {
		return "", nil, fmt.Errorf("orthrus: hash auth key: %w", err)
	}
	return plainKey, hash, nil
}

// Provision creates a new agent and returns it along with the plaintext auth key.
// The key is only available at this point and is not stored in plaintext.
func (s *OrthrusService) Provision(name string) (models.OrthrusAgent, string, error) {
	plainKey, hash, err := newAuthKey()
	if err != nil {
		return models.OrthrusAgent{}, "", err
	}

	agent := models.OrthrusAgent{
		Name:        name,
		AuthKeyHash: string(hash),
		Status:      models.OrthrusStatusPending,
	}

	if err := s.db.Create(&agent).Error; err != nil {
		return models.OrthrusAgent{}, "", fmt.Errorf("orthrus: create agent: %w", err)
	}

	return agent, plainKey, nil
}

// Get retrieves a single OrthrusAgent by UUID.
func (s *OrthrusService) Get(uuid string) (*models.OrthrusAgent, error) {
	var agent models.OrthrusAgent
	if err := s.db.Where("uuid = ?", uuid).First(&agent).Error; err != nil {
		return nil, fmt.Errorf("orthrus: get agent %s: %w", uuid, err)
	}
	return &agent, nil
}

// AgentPatch is a partial update to an agent. Only non-nil fields are written.
type AgentPatch struct {
	Name              *string
	HecateTunnelUUID  *string
	DeviceID          *string
	ResolvedAddress   *string
	ExternalProxyPort *int
	ExternalProxyBind *string
	WriteEnabled      *bool
}

// Patch applies a partial update to an agent.
//
// WriteEnabled has no range validation (unlike ExternalProxyPort) since a
// bool has no invalid values; the audit entry for this specific field is
// emitted by the caller (OrthrusHandler.Patch), not here, matching this
// codebase's existing convention of logging admin-initiated audit events at
// the handler layer (see e.g. security_handler.go, crowdsec_handler.go)
// rather than growing OrthrusService's dependencies to include
// *services.SecurityService and a *gin.Context-derived actor.
func (s *OrthrusService) Patch(uuid string, p AgentPatch) (*models.OrthrusAgent, error) {
	updates := map[string]interface{}{}
	if p.Name != nil {
		trimmed := strings.TrimSpace(*p.Name)
		if trimmed == "" {
			return nil, fmt.Errorf("orthrus: agent name cannot be blank")
		}
		updates["name"] = trimmed
	}
	if p.HecateTunnelUUID != nil {
		updates["hecate_tunnel_uuid"] = *p.HecateTunnelUUID
	}
	if p.DeviceID != nil {
		updates["device_id"] = *p.DeviceID
	}
	if p.ResolvedAddress != nil {
		updates["resolved_address"] = *p.ResolvedAddress
	}
	if p.ExternalProxyPort != nil {
		port := *p.ExternalProxyPort
		if port != 0 && (port < 1024 || port > 65535) {
			return nil, fmt.Errorf("external_proxy_port must be 0 (disabled) or in range 1024–65535, got %d", port)
		}
		updates["external_proxy_port"] = port
	}
	if p.ExternalProxyBind != nil {
		bind, err := normalizeExternalProxyBind(*p.ExternalProxyBind)
		if err != nil {
			return nil, fmt.Errorf("orthrus: patch external proxy bind: %w", err)
		}
		updates["external_proxy_bind"] = bind
	}
	if p.WriteEnabled != nil {
		updates["write_enabled"] = *p.WriteEnabled
	}
	if len(updates) == 0 {
		return s.Get(uuid)
	}
	if err := s.db.Model(&models.OrthrusAgent{}).Where("uuid = ?", uuid).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("orthrus: patch agent %s: %w", uuid, err)
	}
	return s.Get(uuid)
}

// Rename updates the display name of an agent.
func (s *OrthrusService) Rename(uuid, newName string) (*models.OrthrusAgent, error) {
	return s.Patch(uuid, AgentPatch{Name: &newName})
}

// Delete removes an agent from the database (does not revoke/disconnect first).
func (s *OrthrusService) Delete(uuid string) error {
	if err := s.db.Where("uuid = ?", uuid).Delete(&models.OrthrusAgent{}).Error; err != nil {
		return fmt.Errorf("orthrus: delete agent %s: %w", uuid, err)
	}
	return nil
}

// Revoke invalidates an agent's auth key by replacing the hash with an
// unguessable value, then disconnects any active session.
func (s *OrthrusService) Revoke(uuid string) error {
	invalidBytes := make([]byte, 32)
	if _, err := rand.Read(invalidBytes); err != nil {
		return fmt.Errorf("orthrus: generate revoke token: %w", err)
	}
	invalidHash, err := bcrypt.GenerateFromPassword(invalidBytes[:32], orthrus.BcryptCost)
	if err != nil {
		return fmt.Errorf("orthrus: hash revoke token: %w", err)
	}

	if err := s.db.Model(&models.OrthrusAgent{}).
		Where("uuid = ?", uuid).
		Update("auth_key_hash", string(invalidHash)).Error; err != nil {
		return fmt.Errorf("orthrus: revoke agent %s: %w", uuid, err)
	}

	s.disconnect(uuid)
	return nil
}

// RotateKey replaces an agent's auth key and returns the new plaintext key.
// The key is only available at this point. The old key stops working at once and
// any live session is disconnected, so the agent must reconnect with the new key.
// The UUID, name, provider links and proxy settings are unchanged.
func (s *OrthrusService) RotateKey(uuid string) (string, error) {
	plainKey, hash, err := newAuthKey()
	if err != nil {
		return "", err
	}

	res := s.db.Model(&models.OrthrusAgent{}).
		Where("uuid = ?", uuid).
		Update("auth_key_hash", string(hash))
	if res.Error != nil {
		return "", fmt.Errorf("orthrus: rotate key for agent %s: %w", uuid, res.Error)
	}
	if res.RowsAffected == 0 {
		return "", fmt.Errorf("orthrus: rotate key for agent %s: %w", uuid, gorm.ErrRecordNotFound)
	}

	s.disconnect(uuid)
	return plainKey, nil
}

// disconnect closes the agent's live session if one exists and is safe to call
// without a server. A close error is ignored: the session is removed from the
// live table before it is closed, and the stored key has already changed.
func (s *OrthrusService) disconnect(uuid string) {
	if s.server == nil {
		return
	}
	_ = s.server.DisconnectAgent(uuid)
}

// wsURL converts an http/https base URL to the canonical Orthrus WebSocket
// endpoint URL (wss://host/api/v1/ws/orthrus/connect). If the input already
// uses a ws/wss scheme it is returned unchanged (path appended if missing).
func wsURL(base string) string {
	switch {
	case strings.HasPrefix(base, "https://"):
		base = "wss://" + strings.TrimPrefix(base, "https://")
	case strings.HasPrefix(base, "http://"):
		base = "ws://" + strings.TrimPrefix(base, "http://")
	}
	base = strings.TrimRight(base, "/")
	if !strings.HasSuffix(base, "/api/v1/ws/orthrus/connect") {
		base += "/api/v1/ws/orthrus/connect"
	}
	return base
}

// orthrusCGNAT is the carrier-grade NAT range (RFC 6598) used by Tailscale.
var orthrusCGNAT = netip.MustParsePrefix("100.64.0.0/10")

// orthrusPrivateSuffixes are private-use name suffixes that cannot resolve to
// public hosts (mDNS, home.arpa, internal).
var orthrusPrivateSuffixes = []string{".local", ".home.arpa", ".internal"}

// isPrivateAgentHost reports whether an agent may use an unencrypted ws:// link
// to host without an explicit opt-in. It mirrors the agent's rule in agent/main.go;
// the two copies are kept in step by table tests on both sides.
func isPrivateAgentHost(host string) bool {
	host = strings.TrimSuffix(strings.ToLower(host), ".")
	if host == "" {
		return false
	}
	if addr, err := netip.ParseAddr(host); err == nil {
		addr = addr.Unmap()
		return addr.IsLoopback() || addr.IsPrivate() || orthrusCGNAT.Contains(addr)
	}
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return true
	}
	if !strings.Contains(host, ".") {
		return true
	}
	for _, suffix := range orthrusPrivateSuffixes {
		if strings.HasSuffix(host, suffix) {
			return true
		}
	}
	return false
}

// insecureLinkNote returns commented lines for a snippet when serverURL is a
// ws:// link to a host the agent would refuse without the opt-in. The lines use
// the given indent and end with a newline; otherwise it returns "".
func insecureLinkNote(serverURL, indent string) string {
	u, err := url.Parse(serverURL)
	if err != nil || u.Scheme != "ws" || isPrivateAgentHost(u.Hostname()) {
		return ""
	}
	return indent + "# This address is not private, so the agent needs wss:// (Charon behind HTTPS) or ORTHRUS_ALLOW_INSECURE_WS=true.\n" +
		indent + "# - ORTHRUS_ALLOW_INSECURE_WS=true\n"
}

const (
	// agentImageRepo is the Docker Hub and GHCR repository for the agent image.
	agentImageRepo = "wikid82/orthrus"
	// agentLatestTag is the unpinned fallback for builds without a release version.
	agentLatestTag = "latest"
)

// agentImageTag maps a build version to the agent image tag that install
// snippets reference. pinned is true only for plain release versions, which
// have a matching X.Y.Z image. Pre-releases and unparsable input fall back to
// latest because only plain X.Y.Z agent images are published.
func agentImageTag(v string) (tag string, pinned bool) {
	if _, ok := version.Canonical(v); !ok {
		return agentLatestTag, false
	}
	base, meta, hasMeta := strings.Cut(v, "+")
	if strings.Contains(base, "-") {
		return agentLatestTag, false
	}
	if !hasMeta {
		return base, true
	}
	switch {
	case strings.HasPrefix(meta, "dev."):
		return "dev", false
	case strings.HasPrefix(meta, "nightly."):
		return "nightly", false
	default:
		return agentLatestTag, false
	}
}

// agentImageNote returns the comment shown above the image line for tag, or ""
// when the tag needs no explanation.
func agentImageNote(tag string) string {
	switch tag {
	case "dev":
		return "Development build: this tag follows the development branch."
	case "nightly":
		return "Nightly build: this tag follows the nightly channel."
	case agentLatestTag:
		return "This Charon build has no release version; use the agent tag that matches your Charon release."
	default:
		return ""
	}
}

// commentLine renders note as a YAML comment at indent, ending with a newline.
// It returns "" when note is empty.
func commentLine(note, indent string) string {
	if note == "" {
		return ""
	}
	return indent + "# " + note + "\n"
}

// GetInstallSnippets returns platform-specific install templates for an agent.
// The placeholder "<AUTH_KEY>" is used in place of the actual key, which is
// only provided once at Provision time.
func (s *OrthrusService) GetInstallSnippets(uuid, charonURL string) (*orthrus.InstallSnippets, error) {
	agent, err := s.Get(uuid)
	if err != nil {
		return nil, err
	}

	name := agent.Name
	agentUUID := agent.UUID
	serverURL := wsURL(charonURL)
	tag, _ := agentImageTag(version.Version)
	image := agentImageRepo + ":" + tag
	imageNote := agentImageNote(tag)

	return &orthrus.InstallSnippets{
		// The agent drops from root to PUID/PGID at startup (agent/privdrop), so
		// every snippet starts it as root. PGID must equal the host Docker
		// socket's group GID: `stat -c %g /var/run/docker.sock`.
		// The key is written to a file secret rather than an environment variable,
		// so it does not appear in `docker inspect` output or the process environment.
		// The first <AUTH_KEY> must be the printf line: the wizard fills only the first match.
		DockerCompose: fmt.Sprintf(`# Save the auth key next to this file first (Charon shows it only once):
#   printf '%%s' '<AUTH_KEY>' > orthrus_auth_key && chmod 600 orthrus_auth_key
services:
  %s:
%s    image: %s
    restart: always
    container_name: orthrus
    environment:
      # Run as this user/group. Replace <DOCKER_GID> with the output of: stat -c %%g /var/run/docker.sock
      - PUID=65532
      - PGID=<DOCKER_GID>
      - TZ=UTC
      - ORTHRUS_SERVER_URL=%s
%s      - ORTHRUS_AGENT_ID=%s
      # The agent reads the key from this file before it drops privileges.
      - ORTHRUS_AUTH_KEY_FILE=/run/secrets/orthrus_auth_key
    secrets:
      - orthrus_auth_key
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
secrets:
  orthrus_auth_key:
    file: ./orthrus_auth_key
`, name, commentLine(imageNote, "    "), image, serverURL, insecureLinkNote(serverURL, "      "), agentUUID),

		// The key lives in a Secret mounted as a file. fsGroup and defaultMode 0440
		// let the non-root container user read it; the Secret is created separately.
		KubernetesDaemonSet: fmt.Sprintf(`# Create the Secret first (Charon shows the key only once):
#   kubectl create secret generic orthrus-auth-key --from-literal=auth-key='<AUTH_KEY>'
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: orthrus-agent
spec:
  selector:
    matchLabels:
      app: orthrus-agent
  template:
    metadata:
      labels:
        app: orthrus-agent
    spec:
      # The container starts non-root, so the agent skips privilege dropping and
      # PUID/PGID are not needed. The socket group GID (<DOCKER_GID>) is granted
      # as a supplemental group so the process can open the mounted socket.
      securityContext:
        runAsNonRoot: true
        runAsUser: 65532
        runAsGroup: 65532
        supplementalGroups:
          - <DOCKER_GID>
        # Group ownership of the mounted Secret files (the auth key) for the non-root user.
        fsGroup: 65532
      containers:
        - name: agent
%s          image: %s
          env:
            - name: ORTHRUS_SERVER_URL
              value: "%s"
%s            - name: ORTHRUS_AGENT_ID
              value: "%s"
            # The agent reads the key from this file before it drops privileges.
            - name: ORTHRUS_AUTH_KEY_FILE
              value: "/var/run/secrets/orthrus/auth-key"
          securityContext:
            allowPrivilegeEscalation: false
            readOnlyRootFilesystem: true
            capabilities:
              drop:
                - ALL
          volumeMounts:
            - name: docker-sock
              mountPath: /var/run/docker.sock
              readOnly: true
            - name: auth-key
              mountPath: /var/run/secrets/orthrus
              readOnly: true
      volumes:
        - name: docker-sock
          hostPath:
            path: /var/run/docker.sock
            type: Socket
        - name: auth-key
          secret:
            secretName: orthrus-auth-key
            defaultMode: 0440
`, commentLine(imageNote, "          "), image, serverURL, insecureLinkNote(serverURL, "            "), agentUUID),
	}, nil
}

// normalizeExternalProxyBind validates a listen setting and returns its stored
// form: trimmed, with a subnet masked. Reachability is not checked here,
// because interfaces can change; the bind result is reported in proxy-status.
func normalizeExternalProxyBind(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	if len(s) > maxExternalProxyBindLen {
		return "", ErrInvalidExternalProxyBind
	}
	mode, addr, prefix, err := orthrus.ParseBindSetting(s)
	if err != nil {
		return "", ErrInvalidExternalProxyBind
	}
	switch mode {
	case orthrus.BindModeAuto:
		return "", nil
	case orthrus.BindModeSubnet:
		return prefix.String(), nil
	default: // all interfaces or one address
		return addr.String(), nil
	}
}
