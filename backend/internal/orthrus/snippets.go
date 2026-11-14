package orthrus

// InstallSnippets contains per-platform install templates for an Orthrus agent.
// Each snippet uses the literal placeholder "<AUTH_KEY>" wherever the real key
// must be substituted by the user. The real key is only returned once at
// provisioning time and is never embedded in snippet responses.
type InstallSnippets struct {
	DockerCompose       string `json:"docker_compose"`
	KubernetesDaemonSet string `json:"kubernetes_daemonset"`
}
