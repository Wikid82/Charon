package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// RedirectStatusCode enumerates the HTTP redirect status codes Charon
// supports for Redirection Hosts. Only 301/302/307/308 are exposed —
// see docs/plans/current_spec.md §5 for why 300/303/304 are excluded.
type RedirectStatusCode int

const (
	RedirectPermanent         RedirectStatusCode = 301 // Moved Permanently
	RedirectFound             RedirectStatusCode = 302 // Found (temporary)
	RedirectTemporaryPreserve RedirectStatusCode = 307 // Temporary Redirect (method-preserving)
	RedirectPermanentPreserve RedirectStatusCode = 308 // Permanent Redirect (method-preserving)
)

// ValidRedirectStatusCodes is the bounded set accepted by validation.
var ValidRedirectStatusCodes = map[int]bool{
	301: true,
	302: true,
	307: true,
	308: true,
}

// RedirectionHost represents a domain (or set of domains) that Charon
// terminates TLS for and immediately redirects to a target URL, instead
// of reverse-proxying to a backend. Peer resource to ProxyHost — see
// docs/plans/current_spec.md §3 for why this is a separate model.
type RedirectionHost struct {
	ID   uint   `json:"-" gorm:"primaryKey"`
	UUID string `json:"uuid" gorm:"uniqueIndex;not null"`
	Name string `json:"name" gorm:"index"`

	// DomainNames: comma-separated, same convention as ProxyHost.DomainNames.
	DomainNames string `json:"domain_names" gorm:"not null;index"`

	// TargetURL: full target URL, e.g. "https://newsite.example.com" or
	// "https://newsite.example.com/new-path". Scheme is part of the URL,
	// unlike ProxyHost which splits ForwardScheme/ForwardHost/ForwardPort —
	// a redirect target is a browser-facing URL, not a dial address.
	TargetURL string `json:"target_url" gorm:"not null"`

	// StatusCode: one of 301, 302, 307, 308. Validated against
	// ValidRedirectStatusCodes at the service layer; no DB-level CHECK
	// constraint (SQLite CHECK support via GORM is inconsistent across
	// migrations run against pre-existing DBs, so this stays app-level,
	// consistent with how ProxyHost.ForwardScheme is validated).
	StatusCode int `json:"status_code" gorm:"not null;default:301"`

	// PreservePath: when true, the incoming request's path+query string is
	// appended to TargetURL via Caddy's {http.request.uri} placeholder.
	// When false, TargetURL is used verbatim regardless of the incoming path.
	//
	// Deliberately NOT tagged gorm:"default:true": a non-pointer bool set to
	// false is indistinguishable from an unset zero value, so a `default:`
	// tag silently overrides any explicit false on INSERT (the well-known
	// GORM bool-zero-value/default-tag collision). The "default true when
	// omitted" behavior is instead applied at the handler layer
	// (RedirectionHostHandler.Create), which has access to the raw request
	// payload and can tell "omitted" apart from "explicitly false" before
	// the value ever reaches GORM. See redirection_host_handler_test.go's
	// TestRedirectionHostHandler_Create_PersistsExplicitFalseBooleans.
	PreservePath bool `json:"preserve_path"`

	SSLForced    bool `json:"ssl_forced"`
	HTTP2Support bool `json:"http2_support"`

	// HSTS mirrors ProxyHost's fields for consistency — a redirecting
	// domain still terminates HTTPS and can reasonably advertise HSTS.
	HSTSEnabled    bool `json:"hsts_enabled" gorm:"default:false"`
	HSTSSubdomains bool `json:"hsts_subdomains" gorm:"default:false"`

	Enabled bool `json:"enabled" gorm:"default:true;index"`

	// Certificate: same FK pattern as ProxyHost.CertificateID/Certificate.
	CertificateID *uint           `json:"certificate_id" gorm:"index"`
	Certificate   *SSLCertificate `json:"certificate" gorm:"foreignKey:CertificateID"`

	// DNS Challenge configuration — same pattern as ProxyHost, needed
	// because wildcard redirect-source domains still need DNS-01 issuance.
	DNSProviderID   *uint        `json:"dns_provider_id,omitempty" gorm:"index"`
	DNSProvider     *DNSProvider `json:"dns_provider,omitempty" gorm:"foreignKey:DNSProviderID"`
	UseDNSChallenge bool         `json:"use_dns_challenge" gorm:"default:false"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// BeforeCreate assigns a UUID if one has not been set (mirrors ProxyGroup).
func (r *RedirectionHost) BeforeCreate(tx *gorm.DB) (err error) {
	if r.UUID == "" {
		r.UUID = uuid.New().String()
	}
	return
}
