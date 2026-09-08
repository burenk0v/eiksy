package securestorage

func SessionPasswordKey(profileID string) string {
	return "session-password:" + profileID
}

func SessionKeyPassphraseKey(profileID string) string {
	return "session-key-passphrase:" + profileID
}

func VaultTokenKey() string {
	return "settings:vault-token"
}

func KeePassPasswordKey() string {
	return "settings:keepass-password"
}

func AIProviderTokenKey(providerID string) string {
	return "ai-provider-token:" + providerID
}
