package pipeline

import (
	"context"
	"errors"
	"github.com/rancher/rancher/pkg/controllers/user/pipeline/remote"
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

	pipelineLifecycle := &PipelineLifecycle{
		cluster: cluster,
	}
	s := &CronSyncer{
		pipelineLister:     pipelineLister,
		pipelines:          pipelines,
		pipelineExecutions: pipelineExecutions,
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
	pipelineLister := l.cluster.Management.Management.Pipelines("").Controller().Lister()
	pipeline, err := pipelineLister.Get(obj.Namespace, obj.Name)
	if err != nil {
		return obj, err
	}
	if (obj.Spec.TriggerCronExpression != pipeline.Spec.TriggerCronExpression) ||
		(obj.Spec.TriggerCronTimezone != pipeline.Spec.TriggerCronTimezone) {
		//cron trigger changed, reset
		obj.Status.NextStart = ""
	}
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

	id, err := remote.CreateHook(obj, accessToken)
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
