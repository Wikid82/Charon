package services

import (
	"strings"

	"github.com/Wikid82/charon/backend/internal/models"
)

// ScoreBreakdown represents the detailed score calculation
type ScoreBreakdown struct {
	TotalScore  int            `json:"score"`
	MaxScore    int            `json:"max_score"`
	Breakdown   map[string]int `json:"breakdown"`
	Suggestions []string       `json:"suggestions"`
}

// CalculateSecurityScore calculates the security score for a profile
func CalculateSecurityScore(profile *models.SecurityHeaderProfile) ScoreBreakdown {
	breakdown := make(map[string]int)
	suggestions := []string{}
	maxScore := 100

	// HSTS (25 points max)
	hstsScore := 0
	if profile.HSTSEnabled {
		hstsScore += 10
		if profile.HSTSMaxAge >= 31536000 {
			hstsScore += 5
		} else {
			suggestions = append(suggestions, "Increase HSTS max-age to at least 1 year")
		}
		if profile.HSTSIncludeSubdomains {
			hstsScore += 5
		} else {
			suggestions = append(suggestions, "Enable HSTS for subdomains")
		}
		if profile.HSTSPreload {
			hstsScore += 5
		} else {
			suggestions = append(suggestions, "Consider HSTS preload for browser preload lists")
		}
	} else {
		suggestions = append(suggestions, "Enable HSTS to enforce HTTPS")
	}
	breakdown["hsts"] = hstsScore

	// CSP (25 points max)
	cspScore := 0
	if profile.CSPEnabled {
		cspScore += 15
		// Additional points for strict CSP
		if !strings.Contains(profile.CSPDirectives, "'unsafe-inline'") {
			cspScore += 5
		} else {
			suggestions = append(suggestions, "Avoid 'unsafe-inline' in CSP for better security")
		}
		if !strings.Contains(profile.CSPDirectives, "'unsafe-eval'") {
			cspScore += 5
		} else {
			suggestions = append(suggestions, "Avoid 'unsafe-eval' in CSP for better security")
		}
	} else {
		suggestions = append(suggestions, "Enable Content-Security-Policy")
	}
	breakdown["csp"] = cspScore

	// X-Frame-Options (10 points)
	xfoScore := 0
	switch profile.XFrameOptions {
	case "DENY":
		xfoScore = 10
	case "SAMEORIGIN":
		xfoScore = 7
	default:
		suggestions = append(suggestions, "Set X-Frame-Options to DENY or SAMEORIGIN")
	}
	breakdown["x_frame_options"] = xfoScore

	// X-Content-Type-Options (10 points)
	xctoScore := 0
	if profile.XContentTypeOptions {
		xctoScore = 10
	} else {
		suggestions = append(suggestions, "Enable X-Content-Type-Options: nosniff")
	}
	breakdown["x_content_type_options"] = xctoScore

	// Referrer-Policy (10 points)
	rpScore := 0
	strictPolicies := []string{"no-referrer", "strict-origin", "strict-origin-when-cross-origin"}
	for _, p := range strictPolicies {
		if profile.ReferrerPolicy == p {
			rpScore = 10
			break
		}
	}
	if profile.ReferrerPolicy == "origin-when-cross-origin" {
		rpScore = 7
	}
	if rpScore == 0 && profile.ReferrerPolicy != "" {
		rpScore = 3
	}
	if rpScore < 10 {
		suggestions = append(suggestions, "Use a stricter Referrer-Policy")
	}
	breakdown["referrer_policy"] = rpScore

	// Permissions-Policy (10 points)
	ppScore := 0
	if profile.PermissionsPolicy != "" {
		ppScore = 10
	} else {
		suggestions = append(suggestions, "Add Permissions-Policy to restrict browser features")
	}
	breakdown["permissions_policy"] = ppScore

	// Cross-Origin headers (10 points)
	coScore := 0
	if profile.CrossOriginOpenerPolicy != "" {
		coScore += 4
	}
	if profile.CrossOriginResourcePolicy != "" {
		coScore += 3
	}
	if profile.CrossOriginEmbedderPolicy != "" {
		coScore += 3
	}
	if coScore < 10 {
		suggestions = append(suggestions, "Add Cross-Origin isolation headers")
	}
	breakdown["cross_origin"] = coScore

	// Calculate total
	total := hstsScore + cspScore + xfoScore + xctoScore + rpScore + ppScore + coScore

	return ScoreBreakdown{
		TotalScore:  total,
		MaxScore:    maxScore,
		Breakdown:   breakdown,
		Suggestions: suggestions,
	}
}
