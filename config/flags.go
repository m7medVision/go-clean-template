package config

// FeatureFlags is read only by the app. With no file every flag answers its default in code.
type FeatureFlags struct {
	File string `env:"FEATURE_FLAGS_FILE"`
}
