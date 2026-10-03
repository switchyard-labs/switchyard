package actions

// SecretReference identifies a privileged encrypted Worker binding. Values are
// supplied by the operator to Cloudflare, never by repository Action source.
type SecretReference struct {
	Provider string `json:"provider"`
	Binding  string `json:"binding"`
	Purpose  string `json:"purpose"`
}

func WorkerSecretReferences() []SecretReference {
	return []SecretReference{
		{"cloudflare-worker", "CONTROL_SECRET", "Signed control-plane requests"},
		{"cloudflare-worker", "CF_TOKEN", "Repository checkout token minting"},
		{"cloudflare-worker", "R2_ACCESS_KEY_ID", "Bounded CI snapshots"},
		{"cloudflare-worker", "R2_SECRET_ACCESS_KEY", "Bounded CI snapshots"},
	}
}
