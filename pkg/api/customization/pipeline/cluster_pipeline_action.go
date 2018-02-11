package pipeline

import (
	"encoding/json"
	"github.com/pkg/errors"
	"github.com/rancher/norman/api/access"
	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/pipeline/remote/booter"
	"github.com/rancher/rancher/pkg/pipeline/utils"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/client/management/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	"io/ioutil"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"net/http"
	"strings"
)

type ClusterPipelineHandler struct {
	Management config.ManagementContext
}

func ClusterPipelineFormatter(apiContext *types.APIContext, resource *types.RawResource) {
	resource.AddAction(apiContext, "deploy")
	resource.AddAction(apiContext, "destroy")
	resource.AddAction(apiContext, "revokeapp")
	resource.AddAction(apiContext, "authapp")
	resource.AddAction(apiContext, "authuser")
}

func (h *ClusterPipelineHandler) ActionHandler(actionName string, action *types.Action, apiContext *types.APIContext) error {

	//TODO FIXME
	//update endpoint by request url
	if err := utils.UpdateEndpoint(apiContext); err != nil {
		logrus.Errorf("update endpoint got error:%v", err)
	}

	logrus.Infof("get id:%s", apiContext.ID)
	parts := strings.Split(apiContext.ID, ":")
	if len(parts) <= 1 {
		return errors.New("invalid ID")
	}
	ns := parts[0]
	id := parts[1]
	client := h.Management.Management.ClusterPipelines(ns)
	clusterPipeline, err := client.Get(id, metav1.GetOptions{})
	if err != nil {
		return err
	}
	logrus.Infof("do cluster pipeline action:%s", actionName)

	switch actionName {
	case "deploy":
		clusterPipeline.Spec.Deploy = true
	case "destroy":
		clusterPipeline.Spec.Deploy = false
	case "revokeapp":
		return h.revokeapp(apiContext)
	case "authapp":
		return h.authapp(apiContext)
	case "authuser":
		return h.authuser(apiContext)
	}

	_, err = client.Update(clusterPipeline)
	if err != nil {
		return err
	}

	data := map[string]interface{}{}
	if err := access.ByID(apiContext, apiContext.Version, apiContext.Type, apiContext.ID, &data); err != nil {
		return err
	}
	apiContext.WriteResponse(http.StatusOK, data)
	return nil
}

func (h *ClusterPipelineHandler) authapp(apiContext *types.APIContext) error {

	parts := strings.Split(apiContext.ID, ":")
	if len(parts) <= 1 {
		return errors.New("invalid ID")
	}
	ns := parts[0]
	id := parts[1]

	authAppInput := v3.AuthAppInput{}
	requestBytes, err := ioutil.ReadAll(apiContext.Request.Body)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(requestBytes, &authAppInput); err != nil {
		return err
	}
	clusterPipeline, err := h.Management.Management.ClusterPipelines(ns).Get(id, metav1.GetOptions{})
	if err != nil {
		return err
	}

	if authAppInput.Type == "github" {
		clusterPipeline.Spec.GithubConfig = &v3.GithubConfig{
			TLS:          authAppInput.TLS,
			Host:         authAppInput.Host,
			ClientId:     authAppInput.ClientId,
			ClientSecret: authAppInput.ClientSecret,
			RedirectUrl:  authAppInput.RedirectUrl,
		}
	}
	//oauth and add user
	userName := apiContext.Request.Header.Get("Impersonate-User")
	if _, err := h.auth_add_account(clusterPipeline, authAppInput.Type, userName, authAppInput.RedirectUrl, authAppInput.Code); err != nil {
		return err
	}
	//update cluster pipeline config
	if _, err := h.Management.Management.ClusterPipelines(ns).Update(clusterPipeline); err != nil {
		return err
	}

	data := map[string]interface{}{}
	if err := access.ByID(apiContext, apiContext.Version, apiContext.Type, apiContext.ID, &data); err != nil {
		return err
	}

	apiContext.WriteResponse(http.StatusOK, data)
	return nil
}

func (h *ClusterPipelineHandler) authuser(apiContext *types.APIContext) error {

	parts := strings.Split(apiContext.ID, ":")
	if len(parts) <= 1 {
		return errors.New("invalid ID")
	}
	ns := parts[0]
	id := parts[1]

	authUserInput := v3.AuthUserInput{}
	requestBytes, err := ioutil.ReadAll(apiContext.Request.Body)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(requestBytes, &authUserInput); err != nil {
		return err
	}

	clusterPipeline, err := h.Management.Management.ClusterPipelines(ns).Get(id, metav1.GetOptions{})
	if err != nil {
		return err
	}

	if authUserInput.Type == "github" && clusterPipeline.Spec.GithubConfig == nil {
		return errors.New("github oauth app is not configured")
	}

	//oauth and add user
	userName := apiContext.Request.Header.Get("Impersonate-User")
	logrus.Debugf("try auth with %v,%v,%v,%v,%v", clusterPipeline, authUserInput.Type, userName, authUserInput.RedirectUrl, authUserInput.Code)
	account, err := h.auth_add_account(clusterPipeline, authUserInput.Type, userName, authUserInput.RedirectUrl, authUserInput.Code)
	if err != nil {
		return err
	}
	data := map[string]interface{}{}
	if err := access.ByID(apiContext, apiContext.Version, client.SourceCodeCredentialType, account.Name, &data); err != nil {
		return err
	}
	apiContext.WriteResponse(http.StatusOK, data)
	return nil
}

func (h *ClusterPipelineHandler) revokeapp(apiContext *types.APIContext) error {

	parts := strings.Split(apiContext.ID, ":")
	if len(parts) <= 1 {
		return errors.New("invalid ID")
	}
	ns := parts[0]
	id := parts[1]

	clusterPipeline, err := h.Management.Management.ClusterPipelines(ns).Get(id, metav1.GetOptions{})
	if err != nil {
		return err
	}

	if err := h.cleanup(ns, apiContext); err != nil {
		return err
	}

	clusterPipeline.Spec.GithubConfig = nil
	_, err = h.Management.Management.ClusterPipelines(ns).Update(clusterPipeline)
	if err != nil {
		return err
	}

	data := map[string]interface{}{}
	if err := access.ByID(apiContext, apiContext.Version, apiContext.Type, apiContext.ID, &data); err != nil {
		return err
	}

	apiContext.WriteResponse(http.StatusOK, data)
	return nil
}

func (h *ClusterPipelineHandler) auth_add_account(clusterPipeline *v3.ClusterPipeline, remoteType string, userID string, redirectURL string, code string) (*v3.SourceCodeCredential, error) {

	if userID == "" {
		return nil, errors.New("unauth")
	}

	remote, err := booter.New(*clusterPipeline, remoteType)
	if err != nil {
		return nil, err
	}
	account, err := remote.Login(redirectURL, code)
	if err != nil {
		return nil, err
	}
	account.Spec.UserName = userID
	account.Spec.ClusterName = clusterPipeline.Spec.ClusterName
	account, err = h.Management.Management.SourceCodeCredentials("").Create(account)
	if err != nil {
		return nil, err
	}
	return account, nil
}

func (h *ClusterPipelineHandler) cleanup(clusterName string, apiContext *types.APIContext) error {

	//clean resource
	credentialList, err := h.Management.Management.SourceCodeCredentials("").List(metav1.ListOptions{})
	if err != nil {
		return err
	}
	for _, credential := range credentialList.Items {
		if credential.Spec.ClusterName != clusterName {
			continue
		}
		if err := h.Management.Management.SourceCodeCredentials("").DeleteNamespaced(credential.Namespace, credential.Name, &metav1.DeleteOptions{}); err != nil {
			return err
		}
	}

	//TODO filter this cluster
	pipelineList, err := h.Management.Management.Pipelines("").List(metav1.ListOptions{})
	if err != nil {
		return err
	}
	for _, pipeline := range pipelineList.Items {
		if err := h.Management.Management.Pipelines("").DeleteNamespaced(pipeline.Namespace, pipeline.Name, &metav1.DeleteOptions{}); err != nil {
			return err
		}
	}

	//TODO filter this cluster
	executionList, err := h.Management.Management.PipelineExecutions("").List(metav1.ListOptions{})
	if err != nil {
		return err
	}
	for _, execution := range executionList.Items {
		if err := h.Management.Management.PipelineExecutions("").DeleteNamespaced(execution.Namespace, execution.Name, &metav1.DeleteOptions{}); err != nil {
			return err
		}
	}

	//TODO filter this cluster
	logList, err := h.Management.Management.PipelineExecutionLogs("").List(metav1.ListOptions{})
	if err != nil {
		return err
	}
	for _, log := range logList.Items {
		if err := h.Management.Management.PipelineExecutionLogs("").DeleteNamespaced(log.Namespace, log.Name, &metav1.DeleteOptions{}); err != nil {
			return err
		}
	}

	//TODO filter this cluster
	repoList, err := h.Management.Management.SourceCodeRepositories("").List(metav1.ListOptions{})
	if err != nil {
		return err
	}
	for _, repo := range repoList.Items {
		if err := h.Management.Management.SourceCodeRepositories("").DeleteNamespaced(repo.Namespace, repo.Name, &metav1.DeleteOptions{}); err != nil {
			return err
		}
	}

	return nil
}

/*
func (h *ClusterPipelineHandler) test_auth(apiContext *types.APIContext) error {

	parts := strings.Split(apiContext.ID, ":")
	if len(parts) <= 1 {
		return errors.New("invalid ID")
	}
	ns := parts[0]
	id := parts[1]

	requestBody := make(map[string]interface{})
	requestBytes, err := ioutil.ReadAll(apiContext.Request.Body)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(requestBytes, &requestBody); err != nil {
		return err
	}
	var code, remoteType, clientID, clientSecret, redirectURL, scheme, host string

	if requestBody["code"] != nil {
		code = requestBody["code"].(string)
	}
	if requestBody["clientId"] != nil {
		clientID = requestBody["clientId"].(string)
	}
	if requestBody["clientSecret"] != nil {
		clientSecret = requestBody["clientSecret"].(string)
	}
	if requestBody["redirectUrl"] != nil {
		redirectURL = requestBody["redirectUrl"].(string)
	}
	if requestBody["scheme"] != nil {
		scheme = requestBody["scheme"].(string)
	}
	if requestBody["host"] != nil {
		host = requestBody["host"].(string)
	}
	if requestBody["type"] != nil {
		remoteType = requestBody["type"].(string)
	}

	clusterPipeline, err := h.Management.Management.ClusterPipelines(ns).Get(id, metav1.GetOptions{})
	if err != nil {
		return err
	}
	if clientID == "" || clientSecret == "" {

		if clusterPipeline.Spec.GithubConfig == nil {
			return errors.New("github not configured")
		}

		clientID = clusterPipeline.Spec.GithubConfig.ClientId
		clientSecret = clusterPipeline.Spec.GithubConfig.ClientSecret

		//oauth and add user
		userName := apiContext.Request.Header.Get("Impersonate-User")
		if err := h.test_auth_add_account(clusterPipeline, remoteType, userName, redirectURL, code); err != nil {
			return err
		}

	} else {
		if remoteType == "github" && clusterPipeline.Spec.GithubConfig == nil {
			clusterPipeline.Spec.GithubConfig = &v3.GithubConfig{
				Scheme:       scheme,
				Host:         host,
				ClientId:     clientID,
				ClientSecret: clientSecret,
			}
		}
		//oauth and add user
		userName := apiContext.Request.Header.Get("Impersonate-User")
		if err := h.test_auth_add_account(clusterPipeline, remoteType, userName, redirectURL, code); err != nil {
			return err
		}
		//update cluster pipeline config
		if _, err := h.Management.Management.ClusterPipelines(ns).Update(clusterPipeline); err != nil {
			return err
		}
	}
	apiContext.WriteResponse(200, clusterPipeline)
	return nil
}

func (h *ClusterPipelineHandler) test_auth_add_account(clusterPipeline *v3.ClusterPipeline, remoteType string, UserID string, redirectURL string, code string) error {

	account := &v3.RemoteAccount{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "user-" + UserID,
			Name:      "github-test",
		},
		Spec: v3.RemoteAccountSpec{

			Type: "github",
		},
	}
	if UserID == "" {
		return errors.New("unauth")
	}
	account.Spec.UserID = UserID
	if _, err := h.Management.Management.RemoteAccounts("user-" + UserID).Create(account); err != nil {
		return err
	}
	return nil
}
*/
