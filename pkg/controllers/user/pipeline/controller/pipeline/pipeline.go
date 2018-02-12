package pipeline

import (
	"context"
	"errors"
	"github.com/rancher/rancher/pkg/controllers/user/pipeline/remote"
	"github.com/rancher/rancher/pkg/controllers/user/pipeline/utils"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type PipelineLifecycle struct {
	cluster *config.UserContext
}

func Register(ctx context.Context, cluster *config.UserContext) {
	pipelines := cluster.Management.Management.Pipelines("")
	pipelineLister := pipelines.Controller().Lister()
	pipelineExecutions := cluster.Management.Management.PipelineExecutions("")
	pipelineExecutionLister := cluster.Management.Management.PipelineExecutions("").Controller().Lister()

	nodeLister := cluster.Core.Nodes("").Controller().Lister()
	serviceLister := cluster.Core.Services("").Controller().Lister()

	pipelineLifecycle := &PipelineLifecycle{
		cluster: cluster,
	}
	s := &CronSyncer{
		pipelineLister:          pipelineLister,
		pipelines:               pipelines,
		pipelineExecutionLister: pipelineExecutionLister,
		pipelineExecutions:      pipelineExecutions,
		nodeLister:              nodeLister,
		serviceLister:           serviceLister,
	}

	pipelines.AddLifecycle("pipeline-controller", pipelineLifecycle)
	go s.sync(ctx, syncInterval)
}

func (l *PipelineLifecycle) Create(obj *v3.Pipeline) (*v3.Pipeline, error) {

	if obj.Spec.TriggerWebhook && obj.Status.WebHookID == "" {
		id, err := l.createHook(obj)
		if err != nil {
			return obj, err
		}
		obj.Status.WebHookID = id
		if _, err := l.cluster.Management.Management.Pipelines("").Update(obj); err != nil {
			return obj, err
		}
	}
	return obj, nil
}

func (l *PipelineLifecycle) Updated(obj *v3.Pipeline) (*v3.Pipeline, error) {
	return obj, nil
}

func (l *PipelineLifecycle) Remove(obj *v3.Pipeline) (*v3.Pipeline, error) {

	if obj.Status.WebHookID != "" {
		if err := l.deleteHook(obj); err != nil {
			return obj, err
		}
	}
	return obj, nil
}

func (l *PipelineLifecycle) createHook(obj *v3.Pipeline) (string, error) {
	if len(obj.Spec.Stages) <= 0 || len(obj.Spec.Stages[0].Steps) <= 0 || obj.Spec.Stages[0].Steps[0].SourceCodeConfig == nil {
		return "", errors.New("invalid pipeline, missing sourcecode step")
	}
	credentialName := obj.Spec.Stages[0].Steps[0].SourceCodeConfig.SourceCodeCredentialName
	credential, err := l.cluster.Management.Management.SourceCodeCredentials("").Get(credentialName, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	accessToken := credential.Spec.AccessToken
	kind := credential.Spec.SourceCodeType
	mockConfig := v3.ClusterPipeline{
		Spec: v3.ClusterPipelineSpec{
			GithubConfig: &v3.GithubClusterConfig{},
		},
	}
	remote, err := remote.New(mockConfig, kind)
	if err != nil {
		return "", err
	}

	//TODO fixme
	hookUrl := utils.CI_ENDPOINT

	id, err := remote.CreateHook(obj, accessToken, hookUrl)
	if err != nil {
		return "", err
	}
	return id, nil
}

func (l *PipelineLifecycle) deleteHook(obj *v3.Pipeline) error {
	if len(obj.Spec.Stages) <= 0 || len(obj.Spec.Stages[0].Steps) <= 0 || obj.Spec.Stages[0].Steps[0].SourceCodeConfig == nil {
		return errors.New("invalid pipeline, missing sourcecode step")
	}
	credentialName := obj.Spec.Stages[0].Steps[0].SourceCodeConfig.SourceCodeCredentialName
	credential, err := l.cluster.Management.Management.SourceCodeCredentials("").Get(credentialName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	accessToken := credential.Spec.AccessToken
	kind := credential.Spec.SourceCodeType
	mockConfig := v3.ClusterPipeline{
		Spec: v3.ClusterPipelineSpec{
			GithubConfig: &v3.GithubClusterConfig{},
		},
	}
	remote, err := remote.New(mockConfig, kind)
	if err != nil {
		return err
	}

	return remote.DeleteHook(obj, accessToken)
}
