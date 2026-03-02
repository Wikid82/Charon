package routes_test

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

type endpointInventoryEntry struct {
	Name   string
	Method string
	Path   string
	Source string
}

func backendImportRouteMatrix() []endpointInventoryEntry {
	return []endpointInventoryEntry{
		{Name: "Import status", Method: "GET", Path: "/api/v1/import/status", Source: "backend/internal/api/handlers/import_handler.go"},
		{Name: "Import preview", Method: "GET", Path: "/api/v1/import/preview", Source: "backend/internal/api/handlers/import_handler.go"},
		{Name: "Import upload", Method: "POST", Path: "/api/v1/import/upload", Source: "backend/internal/api/handlers/import_handler.go"},
		{Name: "Import upload multi", Method: "POST", Path: "/api/v1/import/upload-multi", Source: "backend/internal/api/handlers/import_handler.go"},
		{Name: "Import detect imports", Method: "POST", Path: "/api/v1/import/detect-imports", Source: "backend/internal/api/handlers/import_handler.go"},
		{Name: "Import commit", Method: "POST", Path: "/api/v1/import/commit", Source: "backend/internal/api/handlers/import_handler.go"},
		{Name: "Import cancel", Method: "DELETE", Path: "/api/v1/import/cancel", Source: "backend/internal/api/handlers/import_handler.go"},
		{Name: "NPM import upload", Method: "POST", Path: "/api/v1/import/npm/upload", Source: "backend/internal/api/handlers/npm_import_handler.go"},
		{Name: "NPM import commit", Method: "POST", Path: "/api/v1/import/npm/commit", Source: "backend/internal/api/handlers/npm_import_handler.go"},
		{Name: "NPM import cancel", Method: "POST", Path: "/api/v1/import/npm/cancel", Source: "backend/internal/api/handlers/npm_import_handler.go"},
		{Name: "JSON import upload", Method: "POST", Path: "/api/v1/import/json/upload", Source: "backend/internal/api/handlers/json_import_handler.go"},
		{Name: "JSON import commit", Method: "POST", Path: "/api/v1/import/json/commit", Source: "backend/internal/api/handlers/json_import_handler.go"},
		{Name: "JSON import cancel", Method: "POST", Path: "/api/v1/import/json/cancel", Source: "backend/internal/api/handlers/json_import_handler.go"},
	}
}

func frontendImportRouteMatrix() []endpointInventoryEntry {
	return []endpointInventoryEntry{
		{Name: "Import status", Method: "GET", Path: "/api/v1/import/status", Source: "frontend/src/api/import.ts"},
		{Name: "Import preview", Method: "GET", Path: "/api/v1/import/preview", Source: "frontend/src/api/import.ts"},
		{Name: "Import upload", Method: "POST", Path: "/api/v1/import/upload", Source: "frontend/src/api/import.ts"},
		{Name: "Import upload multi", Method: "POST", Path: "/api/v1/import/upload-multi", Source: "frontend/src/api/import.ts"},
		{Name: "Import commit", Method: "POST", Path: "/api/v1/import/commit", Source: "frontend/src/api/import.ts"},
		{Name: "Import cancel", Method: "DELETE", Path: "/api/v1/import/cancel", Source: "frontend/src/api/import.ts"},
		{Name: "NPM import upload", Method: "POST", Path: "/api/v1/import/npm/upload", Source: "frontend/src/api/npmImport.ts"},
		{Name: "NPM import commit", Method: "POST", Path: "/api/v1/import/npm/commit", Source: "frontend/src/api/npmImport.ts"},
		{Name: "NPM import cancel", Method: "POST", Path: "/api/v1/import/npm/cancel", Source: "frontend/src/api/npmImport.ts"},
		{Name: "JSON import upload", Method: "POST", Path: "/api/v1/import/json/upload", Source: "frontend/src/api/jsonImport.ts"},
		{Name: "JSON import commit", Method: "POST", Path: "/api/v1/import/json/commit", Source: "frontend/src/api/jsonImport.ts"},
		{Name: "JSON import cancel", Method: "POST", Path: "/api/v1/import/json/cancel", Source: "frontend/src/api/jsonImport.ts"},
	}
}

func saveRouteMatrixForImportWorkflows() []endpointInventoryEntry {
	return []endpointInventoryEntry{
		{Name: "Backup list", Method: "GET", Path: "/api/v1/backups", Source: "frontend/src/api/backups.ts"},
		{Name: "Backup create", Method: "POST", Path: "/api/v1/backups", Source: "frontend/src/api/backups.ts"},
		{Name: "Settings list", Method: "GET", Path: "/api/v1/settings", Source: "frontend/src/api/settings.ts"},
		{Name: "Settings save", Method: "POST", Path: "/api/v1/settings", Source: "frontend/src/api/settings.ts"},
		{Name: "Settings save patch", Method: "PATCH", Path: "/api/v1/settings", Source: "frontend/src/api/settings.ts"},
		{Name: "Settings validate URL", Method: "POST", Path: "/api/v1/settings/validate-url", Source: "frontend/src/api/settings.ts"},
		{Name: "Settings test URL", Method: "POST", Path: "/api/v1/settings/test-url", Source: "frontend/src/api/settings.ts"},
		{Name: "SMTP get", Method: "GET", Path: "/api/v1/settings/smtp", Source: "frontend/src/api/smtp.ts"},
		{Name: "SMTP save", Method: "POST", Path: "/api/v1/settings/smtp", Source: "frontend/src/api/smtp.ts"},
		{Name: "Proxy host list", Method: "GET", Path: "/api/v1/proxy-hosts", Source: "frontend/src/api/proxyHosts.ts"},
		{Name: "Proxy host create", Method: "POST", Path: "/api/v1/proxy-hosts", Source: "frontend/src/api/proxyHosts.ts"},
		{Name: "Proxy host get", Method: "GET", Path: "/api/v1/proxy-hosts/:uuid", Source: "frontend/src/api/proxyHosts.ts"},
		{Name: "Proxy host update", Method: "PUT", Path: "/api/v1/proxy-hosts/:uuid", Source: "frontend/src/api/proxyHosts.ts"},
		{Name: "Proxy host delete", Method: "DELETE", Path: "/api/v1/proxy-hosts/:uuid", Source: "frontend/src/api/proxyHosts.ts"},
	}
}

func backendImportSaveInventoryCanonical() []endpointInventoryEntry {
	entries := append([]endpointInventoryEntry{}, backendImportRouteMatrix()...)
	entries = append(entries, saveRouteMatrixForImportWorkflows()...)
	return entries
}

func frontendObservedImportSaveInventory() []endpointInventoryEntry {
	entries := append([]endpointInventoryEntry{}, frontendImportRouteMatrix()...)
	entries = append(entries, saveRouteMatrixForImportWorkflows()...)
	return entries
}

func routeKey(method, path string) string {
	return method + " " + path
}

func buildRouteLookup(routes []gin.RouteInfo) (map[string]gin.RouteInfo, map[string]map[string]struct{}) {
	byMethodAndPath := make(map[string]gin.RouteInfo, len(routes))
	methodsByPath := make(map[string]map[string]struct{})
	for _, route := range routes {
		key := routeKey(route.Method, route.Path)
		byMethodAndPath[key] = route
		if _, exists := methodsByPath[route.Path]; !exists {
			methodsByPath[route.Path] = map[string]struct{}{}
		}
		methodsByPath[route.Path][route.Method] = struct{}{}
	}
	return byMethodAndPath, methodsByPath
}

func methodList(methodSet map[string]struct{}) []string {
	methods := make([]string, 0, len(methodSet))
	for method := range methodSet {
		methods = append(methods, method)
	}
	sort.Strings(methods)
	return methods
}

func assertStrictMethodPathMatrix(t *testing.T, routes []gin.RouteInfo, expected []endpointInventoryEntry, matrixName string) {
	t.Helper()

	byMethodAndPath, methodsByPath := buildRouteLookup(routes)

	seen := map[string]string{}
	expectedMethodsByPath := map[string]map[string]struct{}{}
	var failures []string

	for _, endpoint := range expected {
		key := routeKey(endpoint.Method, endpoint.Path)
		if previous, duplicated := seen[key]; duplicated {
			failures = append(failures, fmt.Sprintf("duplicate expected entry %q (%s and %s)", key, previous, endpoint.Name))
			continue
		}
		seen[key] = endpoint.Name

		if _, exists := expectedMethodsByPath[endpoint.Path]; !exists {
			expectedMethodsByPath[endpoint.Path] = map[string]struct{}{}
		}
		expectedMethodsByPath[endpoint.Path][endpoint.Method] = struct{}{}

		if _, exists := byMethodAndPath[key]; exists {
			continue
		}

		if methodSet, pathExists := methodsByPath[endpoint.Path]; pathExists {
			failures = append(
				failures,
				fmt.Sprintf("method drift for %s (%s): expected %s, registered methods=[%s]", endpoint.Name, endpoint.Path, endpoint.Method, strings.Join(methodList(methodSet), ", ")),
			)
			continue
		}

		failures = append(
			failures,
			fmt.Sprintf("missing route for %s: expected %s (source=%s)", endpoint.Name, key, endpoint.Source),
		)
	}

	for path, expectedMethodSet := range expectedMethodsByPath {
		actualMethodSet, exists := methodsByPath[path]
		if !exists {
			continue
		}

		extraMethods := make([]string, 0)
		for method := range actualMethodSet {
			if _, expectedMethod := expectedMethodSet[method]; !expectedMethod {
				extraMethods = append(extraMethods, method)
			}
		}
		if len(extraMethods) > 0 {
			sort.Strings(extraMethods)
			failures = append(
				failures,
				fmt.Sprintf(
					"unexpected methods for %s: extra=[%s], expected=[%s], registered=[%s]",
					path,
					strings.Join(extraMethods, ", "),
					strings.Join(methodList(expectedMethodSet), ", "),
					strings.Join(methodList(actualMethodSet), ", "),
				),
			)
		}
	}

	if len(failures) > 0 {
		t.Fatalf("%s route matrix assertion failed:\n- %s", matrixName, strings.Join(failures, "\n- "))
	}
}

func collectRouteMatrixDrift(routes []gin.RouteInfo, expected []endpointInventoryEntry) []string {
	byMethodAndPath, methodsByPath := buildRouteLookup(routes)
	failures := make([]string, 0)

	for _, endpoint := range expected {
		key := routeKey(endpoint.Method, endpoint.Path)
		if _, exists := byMethodAndPath[key]; exists {
			continue
		}

		if methodSet, pathExists := methodsByPath[endpoint.Path]; pathExists {
			failures = append(
				failures,
				fmt.Sprintf("method drift for %s (%s): expected %s, registered methods=[%s]", endpoint.Name, endpoint.Path, endpoint.Method, strings.Join(methodList(methodSet), ", ")),
			)
			continue
		}

		failures = append(
			failures,
			fmt.Sprintf("missing route for %s: expected %s (source=%s)", endpoint.Name, key, endpoint.Source),
		)
	}

	return failures
}
