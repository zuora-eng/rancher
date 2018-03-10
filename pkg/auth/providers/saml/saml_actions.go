package saml

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	"github.com/rancher/types/apis/management.cattle.io/v3"
)

func (s *Provider) formatter(apiContext *types.APIContext, resource *types.RawResource) {
	common.AddCommonActions(apiContext, resource)
	resource.AddAction(apiContext, "configureTest")
	resource.AddAction(apiContext, "testAndApply")
}

func (s *Provider) actionHandler(actionName string, action *types.Action, request *types.APIContext) error {
	handled, err := common.HandleCommonAction(actionName, action, request, s.Name, s.AuthConfigs)
	if err != nil {
		return err
	}
	if handled {
		return nil
	}

	if actionName == "configureTest" {
		return s.configureTest(actionName, action, request)
	} else if actionName == "testAndApply" {
		return s.testAndApply(actionName, action, request)
	}

	return httperror.NewAPIError(httperror.ActionNotAvailable, "")
}

func (s *Provider) configureTest(actionName string, action *types.Action, request *types.APIContext) error {
	samlConfig := &v3.SamlConfig{}
	if err := json.NewDecoder(request.Request.Body).Decode(samlConfig); err != nil {
		return httperror.NewAPIError(httperror.InvalidBodyContent,
			fmt.Sprintf("SAML: Failed to parse body: %v", err))
	}

	redirectURL := s.formSamlRedirectURL(samlConfig)
	err := InitializeSamlClient(samlConfig, s.Name)
	if err != nil {
		return fmt.Errorf("SAML: error in initializing saml client: %v", err)
	}

	data := map[string]interface{}{
		"redirectUrl": redirectURL,
		"type":        "samlConfigTestOutput",
	}

	annotations := make(map[string]string)
	annotations["configured"] = "true"
	samlConfig.Annotations = annotations
	s.saveSamlConfig(samlConfig)
	s.SamlClient.config = samlConfig

	request.WriteResponse(http.StatusOK, data)
	return nil
}

func (s *Provider) formSamlRedirectURL(samlConfig *v3.SamlConfig) string {
	var path string
	if s.Name == PingName {
		path = samlConfig.RancherAPIHost + "/v1-saml/" + PingName + "/login"
	}

	return path
}

func (s *Provider) testAndApply(actionName string, action *types.Action, request *types.APIContext) error {
	var samlConfig v3.SamlConfig
	samlConfigApplyInput := &v3.SamlConfigApplyInput{}

	if err := json.NewDecoder(request.Request.Body).Decode(samlConfigApplyInput); err != nil {
		return httperror.NewAPIError(httperror.InvalidBodyContent,
			fmt.Sprintf("SAML: Failed to parse body: %v", err))
	}

	samlConfig = samlConfigApplyInput.SamlConfig
	s.Request = request
	redirectURL := s.formSamlRedirectURL(&samlConfig)

	http.Redirect(request.Response, request.Request, redirectURL, http.StatusFound)

	fmt.Printf("\n\nYES it's here to set\n\n")
	samlConfig.Enabled = samlConfigApplyInput.Enabled
	err := s.saveSamlConfig(&samlConfig)
	if err != nil {
		return httperror.NewAPIError(httperror.ServerError, fmt.Sprintf("SAML: Failed to save saml config: %v", err))
	}

	return nil
}

//func (s *Provider) testAndApply(actionName string, action *types.Action, request *types.APIContext) error {
//	var samlConfig v3.SamlConfig
//	samlConfigApplyInput := &v3.SamlConfigApplyInput{}
//
//	if err := json.NewDecoder(request.Request.Body).Decode(samlConfigApplyInput); err != nil {
//		return httperror.NewAPIError(httperror.InvalidBodyContent,
//			fmt.Sprintf("SAML: Failed to parse body: %v", err))
//	}
//
//	samlConfig = samlConfigApplyInput.SamlConfig
//	samlLogin := &v3public.CodeBasedLogin{
//		Code: samlConfigApplyInput.Code,
//	}
//
//	//Call provider to testLogin
//	userPrincipal, groupPrincipals, providerInfo, err := s.loginUser(samlLogin, &samlConfig, true)
//	if err != nil {
//		if httperror.IsAPIError(err) {
//			return err
//		}
//		return fmt.Errorf("SAML: server error while authenticating: %v", err)
//	}
//
//	//if this works, save samlConfig CR adding enabled flag
//	user, err := s.UserMGR.SetPrincipalOnCurrentUser(request, userPrincipal)
//	if err != nil {
//		return err
//	}
//
//	samlConfig.Enabled = samlConfigApplyInput.Enabled
//	err = s.saveSamlConfig(&samlConfig)
//	if err != nil {
//		return httperror.NewAPIError(httperror.ServerError, fmt.Sprintf("SAML: Failed to save saml config: %v", err))
//	}
//
//	return tokens.CreateTokenAndSetCookie(user.Name, userPrincipal, groupPrincipals, providerInfo, 0, "Token via Saml Configuration", request)
//}
