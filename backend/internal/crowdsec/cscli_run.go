package crowdsec

import "context"

// RunCSCLI runs cscli with the engine configuration file in front of the arguments, so cscli reads the same
// configuration (and hub directory) as the managed engine. An empty configPath runs cscli without -c.
func RunCSCLI(ctx context.Context, exec CommandExecutor, configPath string, args ...string) ([]byte, error) {
	if configPath != "" {
		args = append([]string{"-c", configPath}, args...)
	}
	return exec.Execute(ctx, "cscli", args...)
}

// runCSCLI runs cscli for the hub service against the configuration file of its data directory.
func (s *HubService) runCSCLI(ctx context.Context, args ...string) ([]byte, error) {
	return RunCSCLI(ctx, s.Exec, FindConfigFile(s.DataDir), args...)
}
