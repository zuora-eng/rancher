package client

const (
	AuthAppInputType              = "authAppInput"
	AuthAppInputFieldClientID     = "clientId"
	AuthAppInputFieldClientSecret = "clientSecret"
	AuthAppInputFieldCode         = "code"
	AuthAppInputFieldHost         = "host"
	AuthAppInputFieldRedirectURL  = "redirectUrl"
	AuthAppInputFieldTLS          = "tls"
	AuthAppInputFieldType         = "type"
)

type AuthAppInput struct {
	ClientID     string `json:"clientId,omitempty"`
	ClientSecret string `json:"clientSecret,omitempty"`
	Code         string `json:"code,omitempty"`
	Host         string `json:"host,omitempty"`
	RedirectURL  string `json:"redirectUrl,omitempty"`
	TLS          *bool  `json:"tls,omitempty"`
	Type         string `json:"type,omitempty"`
}
