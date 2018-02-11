package client

const (
	AuthUserInputType             = "authUserInput"
	AuthUserInputFieldCode        = "code"
	AuthUserInputFieldRedirectURL = "redirectUrl"
	AuthUserInputFieldType        = "type"
)

type AuthUserInput struct {
	Code        string `json:"code,omitempty"`
	RedirectURL string `json:"redirectUrl,omitempty"`
	Type        string `json:"type,omitempty"`
}
