package activities

// getCredential extracts a credential value from a config map by key.
// It checks the nested "auth" sub-map first, then falls back to flat top-level config keys.
// This supports both explicit config["auth"] maps and secret-injected flat keys.
func getCredential(config map[string]interface{}, key string) string {
	if authMap, ok := config["auth"].(map[string]interface{}); ok {
		if v, ok := authMap[key].(string); ok {
			return v
		}
	}
	v, _ := config[key].(string)
	return v
}
