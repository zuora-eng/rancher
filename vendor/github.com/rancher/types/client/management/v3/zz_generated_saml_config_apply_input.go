package client

const (
	SamlConfigApplyInputType            = "samlConfigApplyInput"
	SamlConfigApplyInputFieldCode       = "code"
	SamlConfigApplyInputFieldEnabled    = "enabled"
	SamlConfigApplyInputFieldSamlConfig = "samlConfig"
)

type SamlConfigApplyInput struct {
	Code       string      `json:"code,omitempty" yaml:"code,omitempty"`
	Enabled    bool        `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	SamlConfig *SamlConfig `json:"samlConfig,omitempty" yaml:"samlConfig,omitempty"`
}
