package saml

import (
	"bufio"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/crewjam/saml"
	"github.com/crewjam/saml/logger"
	"github.com/crewjam/saml/samlsp"
	"github.com/gorilla/mux"
	"github.com/rancher/rancher/pkg/auth/tokens"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	log "github.com/sirupsen/logrus"
)

//Client implements a client for the saml library
type Client struct {
	config *v3.SamlConfig
	SamlSP *saml.ServiceProvider
}

type IDPMetadata struct {
	XMLName           xml.Name                `xml:"urn:oasis:names:tc:SAML:2.0:metadata EntityDescriptor"`
	ValidUntil        time.Time               `xml:"validUntil,attr"`
	EntityID          string                  `xml:"entityID,attr"`
	IDPSSODescriptors []saml.IDPSSODescriptor `xml:"IDPSSODescriptor"`
	SPSSODescriptors  []saml.SPSSODescriptor  `xml:"SPSSODescriptor"`
}

const (
	redirectBackBase = "redirectBackBase"
	redirectBackPath = "redirectBackPath"
)

var Root *mux.Router

func InitializeSamlClient(configToSet *v3.SamlConfig, name string) error {

	var idpURL string
	var privKey *rsa.PrivateKey
	var cert *x509.Certificate
	var err error
	var ok bool

	if configToSet.IDPMetadataURL == "" {
		idpURL = ""
		if configToSet.IDPMetadataContent == "" {
			if configToSet.IDPMetadataFilePath == "" {
				log.Debugf("SAML: Cannot initialize saml SP properly, missing IDP URL/metadata in the config %v", configToSet)
			}
		}
	} else {
		idpURL = configToSet.IDPMetadataURL
	}

	if configToSet.SPSelfSignedCert == "" {
		if configToSet.SPSelfSignedCertFilePath != "" {
			cert, err := ioutil.ReadFile(configToSet.SPSelfSignedCertFilePath)
			if err != nil {
				log.Errorf("SAML: Cannot initialize saml SP, cannot read SPSelfSignedCert file in the config %v, error %v", configToSet, err)
			}
			configToSet.SPSelfSignedCert = string(cert)
		} else {
			log.Debugf("SAML: Cannot initialize saml SP properly, missing SPSelfSignedCert in the config %v", configToSet)
		}
	}

	if configToSet.SPSelfSignedKey == "" {
		if configToSet.SPSelfSignedKeyFilePath != "" {
			key, err := ioutil.ReadFile(configToSet.SPSelfSignedKeyFilePath)
			if err != nil {
				return fmt.Errorf("SAML: Cannot initialize saml SP, cannot read SPSelfSignedKey file in the config %v, error %v", configToSet, err)
			}
			configToSet.SPSelfSignedKey = string(key)
		} else {
			log.Debugf("SAML: Cannot initialize saml SP properly, missing SPSelfSignedKey in the config %v", configToSet)
		}
	}

	if configToSet.SPSelfSignedKey != "" {
		// used from ssh.ParseRawPrivateKey

		block, _ := pem.Decode([]byte(configToSet.SPSelfSignedKey))
		if block == nil {
			return fmt.Errorf("SAML: no key found")
		}

		if strings.Contains(block.Headers["Proc-Type"], "ENCRYPTED") {
			return fmt.Errorf("SAML: cannot decode encrypted private keys")
		}

		switch block.Type {
		case "RSA PRIVATE KEY":
			privKey, err = x509.ParsePKCS1PrivateKey(block.Bytes)
			if err != nil {
				return fmt.Errorf("SAML: error parsing PKCS1 RSA key: %v", err)
			}
		case "PRIVATE KEY":
			pk, err := x509.ParsePKCS8PrivateKey(block.Bytes)
			if err != nil {
				return fmt.Errorf("SAML: error parsing PKCS8 RSA key: %v", err)
			}
			privKey, ok = pk.(*rsa.PrivateKey)
			if !ok {
				return fmt.Errorf("SAML: unable to get rsa key")
			}
		default:
			return fmt.Errorf("SAML: unsupported key type %q", block.Type)
		}
	}

	if configToSet.SPSelfSignedCert != "" {
		block, _ := pem.Decode([]byte(configToSet.SPSelfSignedCert))
		if block == nil {
			panic("SAML: failed to parse PEM block containing the private key")
		}

		cert, err = x509.ParseCertificate(block.Bytes)
		if err != nil {
			panic("SAML: failed to parse DER encoded public key: " + err.Error())
		}
	}

	provider := SamlProviders[name]

	samlURL := configToSet.RancherAPIHost + "/v1-saml/"
	samlURL += name
	actURL, err := url.Parse(samlURL)
	if err != nil {
		return fmt.Errorf("SAML: error in parsing URL")
	}

	metadataURL := *actURL
	metadataURL.Path = metadataURL.Path + "/saml/metadata"
	acsURL := *actURL
	acsURL.Path = acsURL.Path + "/saml/acs"

	sp := saml.ServiceProvider{
		Key:         privKey,
		Certificate: cert,
		MetadataURL: metadataURL,
		AcsURL:      acsURL,
		Logger:      logger.DefaultLogger,
	}

	// XML unmarshal throws an error for IdP Metadata cacheDuration field, as it's of type xml Duration. Using a separate struct for unmarshaling for now
	idm := &IDPMetadata{}
	if idpURL != "" {
		tr := &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
		client := &http.Client{Transport: tr}
		resp, err := client.Get(idpURL)
		if err != nil {
			return fmt.Errorf("SAML: cannot initialize saml SP, cannot get IDP Metadata  from the url %v, error %v", idpURL, err)
		}
		sp.IDPMetadata = &saml.EntityDescriptor{}
		if err := xml.NewDecoder(resp.Body).Decode(idm); err != nil {
			return fmt.Errorf("SAML: cannot initialize saml SP, cannot decode IDP Metadata xml from the config %v, error %v", configToSet, err)
		}
	} else if configToSet.IDPMetadataContent != "" {
		sp.IDPMetadata = &saml.EntityDescriptor{}
		if err := xml.NewDecoder(strings.NewReader(configToSet.IDPMetadataContent)).Decode(idm); err != nil {
			return fmt.Errorf("SAML: cannot initialize saml SP, cannot decode IDP Metadata content from the config %v, error %v", configToSet, err)
		}
	} else if configToSet.IDPMetadataFilePath != "" {
		file, err := os.Open(configToSet.IDPMetadataFilePath)
		if err != nil {
			return fmt.Errorf("SAML: cannot initialize saml SP, cannot read IDP Metadata file from the config %v, error %v", configToSet, err)
		}
		metadataReader := bufio.NewReader(file)
		sp.IDPMetadata = &saml.EntityDescriptor{}
		if err := xml.NewDecoder(metadataReader).Decode(idm); err != nil {
			return fmt.Errorf("SAML: cannot initialize saml SP, cannot decode IDP Metadata xml from the config %v, error %v", configToSet, err)
		}
	}

	sp.IDPMetadata.XMLName = idm.XMLName
	sp.IDPMetadata.ValidUntil = idm.ValidUntil
	sp.IDPMetadata.EntityID = idm.EntityID
	sp.IDPMetadata.SPSSODescriptors = idm.SPSSODescriptors
	sp.IDPMetadata.IDPSSODescriptors = idm.IDPSSODescriptors
	provider.SamlClient.SamlSP = &sp

	cookieStore := samlsp.ClientCookies{
		ServiceProvider: &sp,
		Name:            "token",
		Domain:          actURL.Host,
	}
	provider.ClientState = &cookieStore

	if name == PingName {
		Root.Get("PingLogin").HandlerFunc(provider.HandleSamlLogin)
		Root.Get("PingACS").HandlerFunc(provider.ServeHTTP)
		Root.Get("PingMetadata").HandlerFunc(provider.ServeHTTP)
	}
	return nil
}

func PingHandlers() *mux.Router {
	Root = mux.NewRouter()
	Root.Methods("GET").Path("/v1-saml/ping/login").Name("PingLogin")
	Root.Methods("POST").Path("/v1-saml/ping/saml/acs").Name("PingACS")
	Root.Methods("GET").Path("/v1-saml/ping/saml/metadata").Name("PingMetadata")

	provider := SamlProviders[PingName]
	if provider == nil {
		return Root
	}

	storedSamlConfig, err := provider.getSamlConfig()
	if err != nil {
		log.Errorf("SAML(PingHandlers): error in getting config: %v", err)
		return Root
	}
	if !storedSamlConfig.Enabled {
		return Root
	}
	InitializeSamlClient(storedSamlConfig, PingName)
	return Root
}

func (samlClient *Client) getSamlIdentities(samlData map[string][]string) ([]Account, error) {
	//look for saml attributes set in the config
	var samlAccts []Account

	uid, ok := samlData[samlClient.config.UIDField]
	if ok {
		samlAcct := Account{}
		samlAcct.UID = uid[0]

		displayName, ok := samlData[samlClient.config.DisplayNameField]
		if ok {
			samlAcct.DisplayName = displayName[0]
		}

		userName, ok := samlData[samlClient.config.UserNameField]
		if ok {
			samlAcct.UserName = userName[0]
		}
		samlAcct.IsGroup = false

		samlAccts = append(samlAccts, samlAcct)

		groups, ok := samlData[samlClient.config.GroupsField]
		if ok {
			for _, group := range groups {
				groupAcct := Account{}
				groupAcct.UID = group
				groupAcct.IsGroup = true
				groupAcct.DisplayName = group
				samlAccts = append(samlAccts, groupAcct)
			}
		}
	}

	return samlAccts, nil
}

// HandleSamlAssertion processes/handles the assertion obtained by the POST to /saml/acs from IdP
func (s *Provider) HandleSamlAssertion(w http.ResponseWriter, r *http.Request, assertion *saml.Assertion) {
	var groupPrincipals []v3.Principal
	var userPrincipal v3.Principal

	if relayState := r.Form.Get("RelayState"); relayState != "" {
		// delete the cookie
		s.ClientState.DeleteState(w, r, relayState)
	}

	samlData := make(map[string][]string)

	for _, attributeStatement := range assertion.AttributeStatements {
		for _, attr := range attributeStatement.Attributes {
			attrName := attr.FriendlyName
			if attrName == "" {
				attrName = attr.Name
			}
			for _, value := range attr.Values {
				samlData[attrName] = append(samlData[attrName], value.Value)
			}
		}
	}

	config, err := s.getSamlConfig()
	if err != nil {
		log.Errorf("SAML: Error getting saml config %v", err)
		w.WriteHeader(500)
		w.Write([]byte("Server error while authenticating"))
		return
	}
	s.SamlClient.config = config
	accounts, err := s.SamlClient.getSamlIdentities(samlData)
	if err != nil {
		return
	}

	for _, a := range accounts {
		if !a.IsGroup {
			userPrincipal = s.toPrincipal(s.UserType, a, nil)
			userPrincipal.Me = true
		} else {
			groupPrincipal := s.toPrincipal(s.GroupType, a, nil)
			groupPrincipal.MemberOf = true
			groupPrincipals = append(groupPrincipals, groupPrincipal)
		}
	}

	allowedPrincipals := s.SamlClient.config.AllowedPrincipalIDs

	allowed, err := s.UserMGR.CheckAccess(s.SamlClient.config.AccessMode, allowedPrincipals, userPrincipal, groupPrincipals)
	if err != nil {
		log.Errorf("SAML: Error during login while checking access %v", err)
		w.WriteHeader(500)
		w.Write([]byte("Server error while authenticating"))
		return
	}
	if !allowed {
		log.Errorf("SAML: User does not have access %v", err)
		w.WriteHeader(403)
		w.Write([]byte("User does not have access"))
		return
	}

	if !config.Enabled {
		user, err := s.UserMGR.SetPrincipalOnCurrentUser(s.Request, userPrincipal)
		if err != nil {
			log.Errorf("SAML: Error setting principal on current user %v", err)
			w.WriteHeader(500)
			w.Write([]byte("Error setting principal on current user"))
			return
		}

		config.Enabled = true
		err = s.saveSamlConfig(config)
		if err != nil {
			log.Errorf("SAML: Error saving SAML config %v", err)
			w.WriteHeader(500)
			w.Write([]byte(" Error saving SAML config"))
			return
		}

		isSecure := false
		if r.URL.Scheme == "https" {
			isSecure = true
		}
		setRancherToken(w, r, s.TokenMGR, user.Name, userPrincipal, groupPrincipals, isSecure)
		return
	}

	displayName := userPrincipal.DisplayName
	if displayName == "" {
		displayName = userPrincipal.LoginName
	}
	user, err := s.UserMGR.EnsureUser(userPrincipal.Name, displayName)
	if err != nil {
		log.Errorf("SAML: User does not have access %v", err)
		w.WriteHeader(403)
		w.Write([]byte("User does not have access"))
		return
	}

	if p, ok := SamlProviders[s.Name]; ok {
		setRancherToken(w, r, p.PublicTokenMGR, user.Name, userPrincipal, groupPrincipals, true)
	}

	return
}

func setRancherToken(w http.ResponseWriter, r *http.Request, tokenMGR *tokens.Manager, userID string, userPrincipal v3.Principal,
	groupPrincipals []v3.Principal, isSecure bool) {
	rToken, err := tokenMGR.NewLoginToken(userID, userPrincipal, groupPrincipals, map[string]string{}, 0, "")
	if err != nil {
		log.Errorf("Failed creating token with error: %v", err)
		w.WriteHeader(500)
		w.Write([]byte("Server error while authenticating"))
		return
	}
	tokenCookie := &http.Cookie{
		Name:     "R_SESS",
		Value:    rToken.ObjectMeta.Name + ":" + rToken.Token,
		Secure:   isSecure,
		Path:     "/",
		HttpOnly: true,
	}
	http.SetCookie(w, tokenCookie)
	w.WriteHeader(http.StatusOK)
	return
}
