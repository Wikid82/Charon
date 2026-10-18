package crowdsec

import "context"

// withConfigFlag puts `-c <configPath>` in front of cscli arguments so cscli reads the engine configuration
// (and hub directory). An empty configPath leaves the arguments unchanged.
func withConfigFlag(configPath string, args []string) []string {
	if configPath == "" {
		return args
	}
	return append([]string{"-c", configPath}, args...)
}

// RunCSCLI runs cscli with the engine configuration file in front of the arguments, so cscli reads the same
// configuration (and hub directory) as the managed engine. An empty configPath runs cscli without -c.
func RunCSCLI(ctx context.Context, exec CommandExecutor, configPath string, args ...string) ([]byte, error) {
	return exec.Execute(ctx, "cscli", withConfigFlag(configPath, args)...)
}

// runCSCLI runs cscli for the hub service against the configuration file of its data directory.
func (s *HubService) runCSCLI(ctx context.Context, args ...string) ([]byte, error) {
	return RunCSCLI(ctx, s.Exec, FindConfigFile(s.DataDir), args...)
}
