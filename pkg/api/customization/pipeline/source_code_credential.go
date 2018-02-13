package pipeline

import (
	"github.com/rancher/norman/api/access"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/controllers/user/pipeline/remote"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/client/management/v3"
	"github.com/rancher/types/config"
	"github.com/satori/uuid"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"net/http"
)

type SourceCodeCredentialHandler struct {
	Management config.ManagementContext
}

func SourceCodeCredentialFormatter(apiContext *types.APIContext, resource *types.RawResource) {
	resource.AddAction(apiContext, "refreshrepos")
	resource.Links["repos"] = apiContext.URLBuilder.Link("repos", resource)
}

func (h SourceCodeCredentialHandler) LinkHandler(apiContext *types.APIContext, next types.RequestHandler) error {

	if apiContext.Link == "repos" {
		repos, err := h.getReposById(apiContext.ID)
		if err != nil {
			return err
		}
		if len(repos) < 1 {
			return h.refreshrepos(apiContext)
		}

		data := []map[string]interface{}{}
		option := &types.QueryOptions{
			Conditions: []*types.QueryCondition{
				types.NewConditionFromString("sourceCodeCredentialName", types.ModifierEQ, []string{apiContext.ID}...),
			},
		}

		if err := access.List(apiContext, apiContext.Version, client.SourceCodeRepositoryType, option, &data); err != nil {
			return err
		}
		apiContext.Type = client.SourceCodeRepositoryType
		apiContext.WriteResponse(http.StatusOK, data)
		return nil
	}

	return httperror.NewAPIError(httperror.NotFound, "Link not found")
}
func (h *SourceCodeCredentialHandler) ActionHandler(actionName string, action *types.Action, apiContext *types.APIContext) error {
	logrus.Debugf("do sourcecodecredential action:%s", actionName)

	switch actionName {
	case "refreshrepos":
		return h.refreshrepos(apiContext)
	}

	return httperror.NewAPIError(httperror.InvalidAction, "unsupported action")
}

func (h *SourceCodeCredentialHandler) refreshrepos(apiContext *types.APIContext) error {

	_, err := h.refreshReposById(apiContext.ID)
	if err != nil {
		return err
	}
	data := []map[string]interface{}{}
	option := &types.QueryOptions{
		Conditions: []*types.QueryCondition{
			types.NewConditionFromString("sourceCodeCredentialName", types.ModifierEQ, []string{apiContext.ID}...),
		},
	}

	if err := access.List(apiContext, apiContext.Version, client.SourceCodeRepositoryType, option, &data); err != nil {
		return err
	}
	apiContext.Type = client.SourceCodeRepositoryType
	apiContext.WriteResponse(http.StatusOK, data)
	return nil
}

func (h *SourceCodeCredentialHandler) getReposById(sourceCodeCredentialId string) ([]v3.SourceCodeRepository, error) {
	result := []v3.SourceCodeRepository{}
	repoList, err := h.Management.Management.SourceCodeRepositories("").List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	for _, repo := range repoList.Items {
		if repo.Spec.SourceCodeCredentialName == sourceCodeCredentialId {
			result = append(result, repo)
		}
	}
	return result, nil
}

func (h *SourceCodeCredentialHandler) refreshReposById(sourceCodeCredentialId string) ([]v3.SourceCodeRepository, error) {

	credential, err := h.Management.Management.SourceCodeCredentials("").Get(sourceCodeCredentialId, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	remoteType := credential.Spec.SourceCodeType

	mockConfig := v3.ClusterPipeline{
		Spec: v3.ClusterPipelineSpec{
			GithubConfig: &v3.GithubClusterConfig{},
		},
	}
	remote, err := remote.New(mockConfig, remoteType)
	if err != nil {
		return nil, err
	}
	repos, err := remote.Repos(credential)
	if err != nil {
		return nil, err
	}

	//remove old repos
	repoList, err := h.Management.Management.SourceCodeRepositories("").List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	for _, repo := range repoList.Items {
		if repo.Spec.SourceCodeCredentialName == credential.Name {
			if err := h.Management.Management.SourceCodeRepositories("").Delete(repo.Name, &metav1.DeleteOptions{}); err != nil {
				return nil, err
			}
		}
	}

	//store new repos
	for _, repo := range repos {
		repo.Spec.SourceCodeCredentialName = sourceCodeCredentialId
		repo.Spec.ClusterName = credential.Spec.ClusterName
		repo.Spec.UserName = credential.Spec.UserName
		repo.Name = uuid.NewV4().String()
		if _, err := h.Management.Management.SourceCodeRepositories("").Create(&repo); err != nil {
			return nil, err
		}
	}

	return repos, nil
}
