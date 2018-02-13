package pipeline

import (
	"context"
	"errors"
	"github.com/rancher/rancher/pkg/controllers/user/pipeline/remote"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
)

type PipelineLifecycle struct {
	pipelines                  v3.PipelineInterface
	pipelineLister             v3.PipelineLister
	sourceCodeCredentialLister v3.SourceCodeCredentialLister
}

func Register(ctx context.Context, cluster *config.UserContext) {
	pipelines := cluster.Management.Management.Pipelines("")
	pipelineLister := pipelines.Controller().Lister()
	pipelineExecutions := cluster.Management.Management.PipelineExecutions("")
	pipelineExecutionLogs := cluster.Management.Management.PipelineExecutionLogs("")
	sourceCodeCredentialLister := cluster.Management.Management.SourceCodeCredentials("").Controller().Lister()

	pipelineLifecycle := &PipelineLifecycle{
		pipelines:                  pipelines,
		pipelineLister:             pipelineLister,
		sourceCodeCredentialLister: sourceCodeCredentialLister,
	}
	s := &CronSyncer{
		pipelineLister:        pipelineLister,
		pipelines:             pipelines,
		pipelineExecutions:    pipelineExecutions,
		pipelienExecutionLogs: pipelineExecutionLogs,
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
		if _, err := l.pipelines.Update(obj); err != nil {
			return obj, err
		}
	}
	return obj, nil
}

func (l *PipelineLifecycle) Updated(obj *v3.Pipeline) (*v3.Pipeline, error) {
	previous, err := l.pipelineLister.Get(obj.Namespace, obj.Name)
	if err != nil {
		return obj, err
	}
	//handle cron update
	if (obj.Spec.TriggerCronExpression != previous.Spec.TriggerCronExpression) ||
		(obj.Spec.TriggerCronTimezone != previous.Spec.TriggerCronTimezone) {
		//cron trigger changed, reset
		obj.Status.NextStart = ""
	}

	//handle webhook
	if previous.Spec.TriggerWebhook && previous.Status.WebHookID != "" && !obj.Spec.TriggerWebhook {
		if err := l.deleteHook(previous); err != nil {
			logrus.Errorf("fail to delete previous set webhook")
		}
	} else if !previous.Spec.TriggerWebhook && obj.Spec.TriggerWebhook && obj.Status.WebHookID == "" {
		id, err := l.createHook(obj)
		if err != nil {
			return obj, err
		}
		obj.Status.WebHookID = id
		if _, err := l.pipelines.Update(obj); err != nil {
			return obj, err
		}
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
	credential, err := l.sourceCodeCredentialLister.Get("", credentialName)
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
	credential, err := l.sourceCodeCredentialLister.Get("", credentialName)
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
