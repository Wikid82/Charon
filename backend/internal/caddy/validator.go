package caddy

import (
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"strings"
)

// Validate performs pre-flight validation on a Caddy config before applying it.
func Validate(cfg *Config) error {
	if cfg == nil {
		return fmt.Errorf("config cannot be nil")
	}

	if cfg.Apps.HTTP == nil {
		return nil // Empty config is valid
	}

	// Track seen hosts to detect duplicates
	seenHosts := make(map[string]bool)

	for serverName, server := range cfg.Apps.HTTP.Servers {
		if len(server.Listen) == 0 {
			return fmt.Errorf("server %s has no listen addresses", serverName)
		}

		// Validate listen addresses
		for _, addr := range server.Listen {
			if err := validateListenAddr(addr); err != nil {
				return fmt.Errorf("invalid listen address %s in server %s: %w", addr, serverName, err)
			}
		}

		// Validate routes
		for i, route := range server.Routes {
			if err := validateRoute(route, seenHosts); err != nil {
				return fmt.Errorf("invalid route %d in server %s: %w", i, serverName, err)
			}
		}
	}

	// Validate JSON marshalling works
	if _, err := json.Marshal(cfg); err != nil {
		return fmt.Errorf("config cannot be marshalled to JSON: %w", err)
	}

	return nil
}

func validateListenAddr(addr string) error {
	// Strip network type prefix if present (tcp/, udp/)
	if idx := strings.Index(addr, "/"); idx != -1 {
		addr = addr[idx+1:]
	}

	// Parse host:port
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("invalid address format: %w", err)
	}

	// Validate port
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return fmt.Errorf("invalid port: %w", err)
	}
	if port < 1 || port > 65535 {
		return fmt.Errorf("port %d out of range (1-65535)", port)
	}

	// Validate host (allow empty for wildcard binding)
	if host != "" && net.ParseIP(host) == nil {
		return fmt.Errorf("invalid IP address: %s", host)
	}

	return nil
}

func validateRoute(route *Route, seenHosts map[string]bool) error {
	if len(route.Handle) == 0 {
		return fmt.Errorf("route has no handlers")
	}

	// Check for duplicate host matchers
	for _, match := range route.Match {
		for _, host := range match.Host {
			if seenHosts[host] {
				return fmt.Errorf("duplicate host matcher: %s", host)
			}
			seenHosts[host] = true
		}
	}

	// Validate handlers
	for i, handler := range route.Handle {
		if err := validateHandler(handler); err != nil {
			return fmt.Errorf("invalid handler %d: %w", i, err)
		}
	}

	return nil
}

func validateHandler(handler Handler) error {
	handlerType, ok := handler["handler"].(string)
	if !ok {
		return fmt.Errorf("handler missing 'handler' field")
	}

	switch handlerType {
	case "reverse_proxy":
		return validateReverseProxy(handler)
	case "file_server", "static_response":
		return nil // Accept other common handlers
	default:
		// Unknown handlers are allowed (Caddy is extensible)
		return nil
	}
}

func validateReverseProxy(handler Handler) error {
	upstreams, ok := handler["upstreams"].([]map[string]interface{})
	if !ok {
		return fmt.Errorf("reverse_proxy missing upstreams")
	}

	if len(upstreams) == 0 {
		return fmt.Errorf("reverse_proxy has no upstreams")
	}

	for i, upstream := range upstreams {
		dial, ok := upstream["dial"].(string)
		if !ok || dial == "" {
			return fmt.Errorf("upstream %d missing dial address", i)
		}

		// Validate dial address format (host:port)
		if _, _, err := net.SplitHostPort(dial); err != nil {
			return fmt.Errorf("upstream %d has invalid dial address %s: %w", i, dial, err)
		}
	}

	return nil
}
