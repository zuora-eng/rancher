package controller

import (
	"errors"
	"github.com/rancher/rancher/pkg/pipeline/remote/booter"
	"github.com/rancher/rancher/pkg/pipeline/utils"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type PipelineLifecycle struct {
	cluster *config.ClusterContext
}

func (l *PipelineLifecycle) Create(obj *v3.Pipeline) (*v3.Pipeline, error) {

	if obj.Spec.TriggerWebhook && obj.Status.WebHookId == "" {
		id, err := l.createHook(obj)
		if err != nil {
			return obj, err
		}
		obj.Status.WebHookId = id
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

	if obj.Status.WebHookId != "" {
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
			GithubConfig: &v3.GithubConfig{},
		},
	}
	remote, err := booter.New(mockConfig, kind)
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
			GithubConfig: &v3.GithubConfig{},
		},
	}
	remote, err := booter.New(mockConfig, kind)
	if err != nil {
		return err
	}

	return remote.DeleteHook(obj, accessToken)
}
