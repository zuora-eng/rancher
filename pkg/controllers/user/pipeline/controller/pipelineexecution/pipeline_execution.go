package controller

import (
	"errors"
	"fmt"
	"github.com/rancher/rancher/pkg/pipeline/engine"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type PipelineHistoryLifecycle struct {
	cluster *config.ClusterContext
}

func (l *PipelineHistoryLifecycle) Create(obj *v3.PipelineExecution) (*v3.PipelineExecution, error) {

	url, err := l.getJenkinsURL()
	if err != nil {
		logrus.Errorf("Error connect to Jenkins - %v", err)
		return l.errorHistory(obj)
	}
	pipelineEngine, err := engine.New(l.cluster, url)
	if err != nil {
		logrus.Errorf("Error get Jenkins engine - %v", err)
		return l.errorHistory(obj)
	}
	if err := pipelineEngine.RunPipeline(&obj.Spec.Pipeline, obj.Spec.TriggeredBy); err != nil {
		logrus.Errorf("Error run pipeline - %v", err)
		return l.errorHistory(obj)
	}
	return obj, nil
}
func (l *PipelineHistoryLifecycle) errorHistory(obj *v3.PipelineExecution) (*v3.PipelineExecution, error) {
	obj.Status.State = "error"
	if _, err := l.cluster.Management.Management.PipelineExecutions(obj.Namespace).Update(obj); err != nil {
		logrus.Error(err)
		return obj, err
	}
	return obj, nil
}

func (l *PipelineHistoryLifecycle) Updated(obj *v3.PipelineExecution) (*v3.PipelineExecution, error) {
	return obj, nil
}

func (l *PipelineHistoryLifecycle) Remove(obj *v3.PipelineExecution) (*v3.PipelineExecution, error) {
	return obj, nil
}

//FIXME proper way to connect to Jenkins in cluster
func (l *PipelineHistoryLifecycle) getJenkinsURL() (string, error) {

	nodes, err := l.cluster.Core.Nodes("").List(metav1.ListOptions{})
	if err != nil {
		return "", err
	}
	if len(nodes.Items) < 1 {
		return "", errors.New("no available nodes")
	}
	if len(nodes.Items[0].Status.Addresses) < 1 {
		return "", errors.New("no available address")
	}
	host := nodes.Items[0].Status.Addresses[0].Address

	svcport := 0
	service, err := l.cluster.Core.Services("cattle-pipeline").Get("jenkins", metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	ports := service.Spec.Ports
	for _, port := range ports {
		if port.NodePort != 0 && port.Name == "http" {
			svcport = int(port.NodePort)
			break
		}
	}
	return fmt.Sprintf("http://%s:%d", host, svcport), nil
}
