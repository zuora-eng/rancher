package saml

type Account struct {
	UID         string `json:"uid,omitempty"`         //objectId
	DisplayName string `json:"displayname,omitempty"` //name
	UserName    string `json:"username,omitempty"`    //samAccountName (login name)
	IsGroup     bool
}
