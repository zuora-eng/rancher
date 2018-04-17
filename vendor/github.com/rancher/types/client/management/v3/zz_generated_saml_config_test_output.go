package client

const (
	SamlConfigTestOutputType             = "samlConfigTestOutput"
	SamlConfigTestOutputFieldRedirectURL = "redirectUrl"
)

type SamlConfigTestOutput struct {
	RedirectURL string `json:"redirectUrl,omitempty" yaml:"redirectUrl,omitempty"`
}
