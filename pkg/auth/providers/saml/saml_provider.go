package saml

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/apis/management.cattle.io/v3public"
	"github.com/rancher/types/client/management/v3"
	publicclient "github.com/rancher/types/client/management/v3public"
	"github.com/rancher/types/config"
	"github.com/rancher/types/user"
	"github.com/sirupsen/logrus"
	log "github.com/sirupsen/logrus"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const PingName = "ping"

type Provider struct {
	Ctx         context.Context
	AuthConfigs v3.AuthConfigInterface
	UserMGR     user.Manager
	SamlClient  *SamlClient
	Name        string
	UserType    string
	GroupType   string
	RedirectURL string
	Request     *types.APIContext
}

var SamlProviders = make(map[string]*Provider)

func Configure(ctx context.Context, mgmtCtx *config.ScaledContext, userMGR user.Manager, name string) common.AuthProvider {
	samlClient := &SamlClient{}
	samlp := &Provider{
		Ctx:         ctx,
		AuthConfigs: mgmtCtx.Management.AuthConfigs(""),
		UserMGR:     userMGR,
		SamlClient:  samlClient,
		Name:        name,
		UserType:    name + "_user",
		GroupType:   name + "_group",
	}
	SamlProviders[PingName] = samlp
	return samlp
}

func (s *Provider) GetName() string {
	return s.Name
}

func (s *Provider) CustomizeSchema(schema *types.Schema) {
	schema.ActionHandler = s.actionHandler
	schema.Formatter = s.formatter
}

func (s *Provider) TransformToAuthProvider(authConfig map[string]interface{}) map[string]interface{} {
	p := common.TransformToAuthProvider(authConfig)
	if s.Name == PingName {
		p[publicclient.PingProviderFieldRedirectURL] = formSamlRedirectURLFromMap(authConfig, s.Name)
	}
	return p
}

func (s *Provider) AuthenticateUser(input interface{}) (v3.Principal, []v3.Principal, map[string]string, error) {
	login, ok := input.(*v3public.CodeBasedLogin)
	if !ok {
		return v3.Principal{}, nil, nil, fmt.Errorf("SAML: unexpected input type")
	}

	return s.loginUser(login, nil, false)
}

func PerformAuthRedirect(name string, apiContext *types.APIContext) string {
	var redirectURL string

	if provider, ok := SamlProviders[name]; ok {

		savedConfig, err := provider.getSamlConfig()
		if err != nil {
			fmt.Errorf("\nERROR!!: %v\n", err)
		}

		redirectURL = provider.formSamlRedirectURL(savedConfig)
	}

	http.Redirect(apiContext.Response, apiContext.Request, redirectURL, http.StatusFound)
	return ""
}

func (s *Provider) loginUser(samlCredential *v3public.CodeBasedLogin, config *v3.SamlConfig, test bool) (v3.Principal, []v3.Principal, map[string]string, error) {
	var groupPrincipals []v3.Principal
	var userPrincipal v3.Principal
	var err error
	var samlData map[string][]string

	if config == nil {
		config, err = s.getSamlConfig()
		if err != nil {
			return v3.Principal{}, nil, nil, err
		}
	}
	s.SamlClient.config = config
	inputMap := samlCredential.Code

	if err := json.Unmarshal([]byte(inputMap), &samlData); err != nil {
		log.Errorf("SAML: Error getting saml data from input %v", err)
		return v3.Principal{}, nil, nil, err
	}

	accounts, err := s.SamlClient.getSamlIdentities(samlData)
	if err != nil {
		return v3.Principal{}, nil, nil, err
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

	allowedPrincipals := config.AllowedPrincipalIDs
	if test && config.AccessMode == "restricted" {
		allowedPrincipals = append(allowedPrincipals, userPrincipal.Name)
	}

	allowed, err := s.UserMGR.CheckAccess(config.AccessMode, allowedPrincipals, userPrincipal, groupPrincipals)
	if err != nil {
		return v3.Principal{}, nil, nil, err
	}
	if !allowed {
		return v3.Principal{}, nil, nil, httperror.NewAPIError(httperror.Unauthorized, "unauthorized")
	}

	return userPrincipal, groupPrincipals, map[string]string{}, nil
}

func (s *Provider) getSamlConfig() (*v3.SamlConfig, error) {
	authConfigObj, err := s.AuthConfigs.ObjectClient().UnstructuredClient().Get(s.Name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("SAML: failed to retrieve SamlConfig, error: %v", err)
	}

	u, ok := authConfigObj.(runtime.Unstructured)
	if !ok {
		return nil, fmt.Errorf("SAML: failed to retrieve SamlConfig, cannot read k8s Unstructured data")
	}
	storedSamlConfigMap := u.UnstructuredContent()

	storedSamlConfig := &v3.SamlConfig{}
	decode(storedSamlConfigMap, storedSamlConfig)

	if enabled, ok := storedSamlConfigMap["enabled"].(bool); ok {
		storedSamlConfig.Enabled = enabled
	}

	metadataMap, ok := storedSamlConfigMap["metadata"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("SAML: failed to retrieve SamlConfig metadata, cannot read k8s Unstructured data")
	}

	typemeta := &metav1.ObjectMeta{}
	mapstructure.Decode(metadataMap, typemeta)
	storedSamlConfig.ObjectMeta = *typemeta

	return storedSamlConfig, nil
}

func (s *Provider) saveSamlConfig(config *v3.SamlConfig) error {
	var configType string

	storedSamlConfig, err := s.getSamlConfig()
	if err != nil {
		return err
	}

	if s.Name == "ping" {
		configType = client.PingConfigType
	}

	config.APIVersion = "management.cattle.io/v3"
	config.Kind = v3.AuthConfigGroupVersionKind.Kind
	config.Type = configType
	config.ObjectMeta = storedSamlConfig.ObjectMeta

	logrus.Debugf("updating samlConfig")
	_, err = s.AuthConfigs.ObjectClient().Update(config.ObjectMeta.Name, config)
	if err != nil {
		return err
	}
	return nil

}

func (s *Provider) toPrincipal(principalType string, acct Account, token *v3.Token) v3.Principal {
	displayName := acct.DisplayName
	if displayName == "" {
		displayName = acct.UserName
	}

	princ := v3.Principal{
		ObjectMeta:  metav1.ObjectMeta{Name: principalType + "://" + acct.UserName},
		DisplayName: displayName,
		LoginName:   acct.UID,
		Provider:    s.Name,
		Me:          false,
	}

	if principalType == s.UserType {
		princ.PrincipalType = "user"
		if token != nil {
			princ.Me = s.isThisUserMe(token.UserPrincipal, princ)
		}
	} else {
		princ.PrincipalType = "group"
		princ.ObjectMeta = metav1.ObjectMeta{Name: principalType + "://" + acct.UID}
		if token != nil {
			princ.MemberOf = s.isMemberOf(token.GroupPrincipals, princ)
		}
	}

	return princ
}

func (s *Provider) SearchPrincipals(searchKey, principalType string, token v3.Token) ([]v3.Principal, error) {
	var principals []v3.Principal

	if principalType == "" {
		principalType = s.UserType
	}

	p := v3.Principal{
		ObjectMeta:    metav1.ObjectMeta{Name: searchKey + "://" + s.UserType},
		DisplayName:   searchKey,
		LoginName:     searchKey,
		PrincipalType: principalType,
		Provider:      s.Name,
	}

	principals = append(principals, p)

	return principals, nil
}

func (s *Provider) GetPrincipal(principalID string, token v3.Token) (v3.Principal, error) {
	parts := strings.SplitN(principalID, ":", 2)
	if len(parts) != 2 {
		return v3.Principal{}, errors.Errorf("SAML: invalid id %v", principalID)
	}
	scope := parts[0]
	externalID := strings.TrimPrefix(parts[1], "//")

	acct := Account{
		DisplayName: externalID,
		UserName:    externalID,
	}

	p := s.toPrincipal(scope, acct, &token)

	return p, nil
}

//getSamlRedirectURL returns the redirect URL for SAML login flow
func (s *Provider) getSamlRedirectURL(redirectBackBase string, redirectBackPath string) string {
	redirectURL := ""
	storedSamlConfig, err := s.getSamlConfig()
	if err != nil {
		log.Errorf("SAML: error in getting saml config")
		return redirectURL
	}

	if storedSamlConfig != nil {
		rancherAPI := storedSamlConfig.RancherAPIHost
		redirectURL = redirectBackBase + redirectBackPath
		if redirectURL == "" {
			redirectURL = rancherAPI + redirectBackPath
		}
		log.Infof("getSamlRedirectURL : redirectURL %v ", redirectURL)
	}
	return redirectURL
}

func (s *Provider) isThisUserMe(me v3.Principal, other v3.Principal) bool {
	if me.ObjectMeta.Name == other.ObjectMeta.Name && me.LoginName == other.LoginName && me.PrincipalType == other.PrincipalType {
		return true
	}
	return false
}

func (s *Provider) isMemberOf(myGroups []v3.Principal, other v3.Principal) bool {
	for _, mygroup := range myGroups {
		if mygroup.ObjectMeta.Name == other.ObjectMeta.Name && mygroup.PrincipalType == other.PrincipalType {
			return true
		}
	}
	return false
}

func formSamlRedirectURLFromMap(config map[string]interface{}, name string) string {
	hostname, _ := config[client.PingConfigFieldRancherAPIHost].(string)
	path := hostname + "/v1-saml/" + name + "/login"
	return path
}

func decode(m interface{}, rawVal interface{}) error {
	config := &mapstructure.DecoderConfig{
		Metadata: nil,
		Result:   rawVal,
		TagName:  "json",
	}

	decoder, err := mapstructure.NewDecoder(config)
	if err != nil {
		return err
	}

	return decoder.Decode(m)
}
