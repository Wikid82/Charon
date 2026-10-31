package crowdsec

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/util"
)

// withConfigFlag puts `-c <configPath>` in front of cscli arguments so cscli reads the engine configuration
// (and hub directory). An empty configPath leaves the arguments unchanged.
func withConfigFlag(configPath string, args []string) []string {
	if configPath == "" {
		return args
	}
	return append([]string{"-c", configPath}, args...)
}

// subcommand returns the cscli arguments without a leading -c flag pair.
func subcommand(args []string) []string {
	if len(args) >= 2 && args[0] == "-c" {
		return args[2:]
	}
	return args
}

// RunCSCLI runs cscli with the engine configuration file in front of the arguments, so cscli reads the same
// configuration (and hub directory) as the managed engine. An empty configPath runs cscli without -c.
func RunCSCLI(ctx context.Context, exec CommandExecutor, configPath string, args ...string) ([]byte, error) {
	return exec.Execute(ctx, "cscli", withConfigFlag(configPath, args)...)
}

// HubCSCLIOptions says how RunHubCSCLI retries a hub request on the hub mirror.
type HubCSCLIOptions struct {
	// ConfigPath is the live cscli configuration file. An empty path disables the mirror retry.
	ConfigPath string
	// Template is the cscli hub URL template of the mirror (see mirrorHubTemplate). An empty template disables the
	// mirror retry.
	Template string
	// Memory is the mirror-first window. A nil memory is always closed.
	Memory *MirrorMemory
}

// RunHubCSCLI runs a cscli command against the live configuration. A command that downloads from the hub (see
// isHubNetworkOp) is retried once on the hub mirror when the live hub address fails: cscli is run under a derived
// configuration that points its hub at the mirror, and the derived copy is removed on every path. While the mirror
// window of opts.Memory is open, the mirror is tried first and the live address is the single retry. Any other
// command, and any call without a mirror configuration, is a plain RunCSCLI call.
func RunHubCSCLI(ctx context.Context, exec CommandExecutor, opts HubCSCLIOptions, args ...string) ([]byte, error) {
	if opts.ConfigPath == "" || opts.Template == "" || !isHubNetworkOp(args) {
		return RunCSCLI(ctx, exec, opts.ConfigPath, args...)
	}
	if opts.Memory.Active() {
		derived, cleanup, err := mirrorCSCLIConfig(opts.ConfigPath, opts.Template)
		if err == nil {
			defer cleanup()
			return mirrorFirst(ctx, exec, opts, derived, args)
		}
		logMirrorUnavailable(err)
	}
	return liveFirst(ctx, exec, opts, args)
}

// liveFirst runs args on the live address and, on a retryable failure, once on the mirror.
func liveFirst(ctx context.Context, exec CommandExecutor, opts HubCSCLIOptions, args []string) ([]byte, error) {
	out, timedOut, err := runShare(ctx, exec, opts.ConfigPath, args)
	if err == nil || !isRetryableAttempt(ctx, args, out, timedOut, err) {
		return out, err
	}
	logMirrorRetry(args, err)
	derived, cleanup, derr := mirrorCSCLIConfig(opts.ConfigPath, opts.Template)
	if derr != nil {
		logMirrorUnavailable(derr)
		return out, err
	}
	defer cleanup()
	retryOut, retryErr := runOnConfig(ctx, exec, derived, args)
	if retryErr != nil {
		return retryOut, fmt.Errorf("retry on hub mirror: %w; live hub address: %w", retryErr, err)
	}
	opts.Memory.Remember()
	logger.Log().WithField("hub_fallback_used", true).WithField("operation", hubOperation(args)).
		Info("cscli hub request answered by the hub mirror")
	return retryOut, nil
}

// mirrorFirst runs args on the derived mirror configuration and, on a retryable failure, once on the live address.
// A live answer that rescues the request closes the mirror-first window, because the hub address is back.
func mirrorFirst(ctx context.Context, exec CommandExecutor, opts HubCSCLIOptions, derived string, args []string) ([]byte, error) {
	out, timedOut, err := runShare(ctx, exec, derived, args)
	if err == nil || !isRetryableAttempt(ctx, args, out, timedOut, err) {
		return out, err
	}
	logMirrorRetry(args, err)
	liveOut, liveErr := runOnConfig(ctx, exec, opts.ConfigPath, args)
	if liveErr != nil {
		return liveOut, fmt.Errorf("retry on live hub address: %w; hub mirror: %w", liveErr, err)
	}
	opts.Memory.Forget()
	logger.Log().WithField("operation", hubOperation(args)).Info("cscli hub request answered by the live hub address; mirror-first window cleared")
	return liveOut, nil
}

// runShare runs args under this operation's share of the time left on ctx. A hub update gets half of it, so a hung
// hub leaves time for the retry; every other operation gets all of it, so a slow healthy install is not cut short.
// timedOut reports that the share ran out while ctx itself is still alive.
func runShare(ctx context.Context, exec CommandExecutor, configPath string, args []string) (out []byte, timedOut bool, err error) {
	attemptsLeft := 1
	if isHubUpdate(args) {
		attemptsLeft = 2
	}
	attemptCtx, cancel := attemptContext(ctx, attemptsLeft)
	defer cancel()
	out, err = exec.Execute(attemptCtx, "cscli", withConfigFlag(configPath, args)...)
	timedOut = err != nil && attemptCtx.Err() != nil && ctx.Err() == nil
	return out, timedOut, err
}

// isRetryableAttempt reports whether a failed attempt may be retried on the other address. A dead ctx is never
// retried. A hub update is retried on a network failure, a timeout or an expired share. Install and upgrade are
// retried on a network failure only, never on a bare deadline, so a healthy slow install is not killed.
func isRetryableAttempt(ctx context.Context, args []string, out []byte, timedOut bool, err error) bool {
	if ctx.Err() != nil {
		return false
	}
	text := string(out) + " " + err.Error()
	if isHubUpdate(args) {
		return timedOut || isRetryableHubFailure(text)
	}
	return isMirrorRetryableFailure(text)
}

// runOnConfig runs args under configPath. An install or upgrade refreshes the hub index under the same configuration
// first, so the index and the items come from one source; a failed refresh ends the run.
func runOnConfig(ctx context.Context, exec CommandExecutor, configPath string, args []string) ([]byte, error) {
	if !isHubUpdate(args) {
		if out, err := RunCSCLI(ctx, exec, configPath, "hub", "update"); err != nil {
			return out, fmt.Errorf("hub update before retry: %w", err)
		}
	}
	return RunCSCLI(ctx, exec, configPath, args...)
}

// hubURLInText matches a URL inside cscli output, so the log shows it without credentials or query strings.
var hubURLInText = regexp.MustCompile(`https?://[^\s"']+`)

// hubLogText makes cscli output safe for a log line: URLs are reduced by HubURLForLog and control characters are
// replaced.
func hubLogText(text string) string {
	return util.SanitizeForLog(hubURLInText.ReplaceAllStringFunc(text, HubURLForLog))
}

// hubOperation names the hub operation of args for a log line, for example "collections install".
func hubOperation(args []string) string {
	return util.SanitizeForLog(strings.Join(subcommand(args), " "))
}

func logMirrorRetry(args []string, err error) {
	logger.Log().WithField("operation", hubOperation(args)).WithField("error", hubLogText(err.Error())).
		Warn("cscli hub request failed on the live hub address; retrying on the hub mirror")
}

func logMirrorUnavailable(err error) {
	logger.Log().WithField("error", hubLogText(err.Error())).Warn("hub mirror retry unavailable")
}

// runCSCLI runs cscli for the hub service against the configuration file of its data directory.
func (s *HubService) runCSCLI(ctx context.Context, args ...string) ([]byte, error) {
	return RunCSCLI(ctx, s.Exec, FindConfigFile(s.DataDir), args...)
}

// runHubCSCLI runs a hub download for the hub service: it is retried on the mirror as RunHubCSCLI describes.
func (s *HubService) runHubCSCLI(ctx context.Context, args ...string) ([]byte, error) {
	return RunHubCSCLI(ctx, s.Exec, s.MirrorOptions(), args...)
}

// MirrorOptions returns the mirror retry settings of the service: the live configuration of its data directory, the
// mirror template for MirrorBaseURL, and the shared mirror-first window.
func (s *HubService) MirrorOptions() HubCSCLIOptions {
	template := s.mirrorTemplate.get(s.MirrorBaseURL)
	return HubCSCLIOptions{ConfigPath: FindConfigFile(s.DataDir), Template: template, Memory: s.mirror}
}
