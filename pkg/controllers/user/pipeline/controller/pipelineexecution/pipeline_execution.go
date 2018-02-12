package pipelineexecution

import (
	"context"
	"errors"
	"fmt"
	"github.com/rancher/rancher/pkg/controllers/user/pipeline/engine"
	"github.com/rancher/rancher/pkg/controllers/user/pipeline/utils"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type PipelineExecutionLifecycle struct {
	cluster *config.UserContext
}

func Register(ctx context.Context, cluster *config.UserContext) {
	pipelineExecutions := cluster.Management.Management.PipelineExecutions("")
	pipelineExecutionLister := pipelineExecutions.Controller().Lister()
	pipelineExecutionLogs := cluster.Management.Management.PipelineExecutionLogs("")
	pipelineExecutionLogLister := pipelineExecutionLogs.Controller().Lister()

	nodeLister := cluster.Core.Nodes("").Controller().Lister()
	serviceLister := cluster.Core.Services("").Controller().Lister()

	pipelineExecutionLifecycle := &PipelineExecutionLifecycle{
		cluster: cluster,
	}
	stateSyncer := &ExecutionStateSyncer{
		pipelineExecutionLister: pipelineExecutionLister,
		pipelineExecutions:      pipelineExecutions,
		nodeLister:              nodeLister,
		serviceLister:           serviceLister,
	}
	logSyncer := &ExecutionLogSyncer{
		pipelineExecutionLister:    pipelineExecutionLister,
		pipelineExecutionLogLister: pipelineExecutionLogLister,
		pipelineExecutionLogs:      pipelineExecutionLogs,
		nodeLister:                 nodeLister,
		serviceLister:              serviceLister,
	}

	pipelineExecutions.AddLifecycle(pipelineExecutionLifecycle.GetName(), pipelineExecutionLifecycle)

	go stateSyncer.sync(ctx, syncStateInterval)
	go logSyncer.sync(ctx, syncLogInterval)

}

func (l *PipelineExecutionLifecycle) Create(obj *v3.PipelineExecution) (*v3.PipelineExecution, error) {

	if obj.Status.State != utils.StateWaiting {
		return obj, nil
	}
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
func (l *PipelineExecutionLifecycle) errorHistory(obj *v3.PipelineExecution) (*v3.PipelineExecution, error) {
	obj.Status.State = "error"
	if _, err := l.cluster.Management.Management.PipelineExecutions(obj.Namespace).Update(obj); err != nil {
		logrus.Error(err)
		return obj, err
	}
	return obj, nil
}

func (l *PipelineExecutionLifecycle) Updated(obj *v3.PipelineExecution) (*v3.PipelineExecution, error) {
	return obj, nil
}

func (l *PipelineExecutionLifecycle) Remove(obj *v3.PipelineExecution) (*v3.PipelineExecution, error) {
	return obj, nil
}

//FIXME proper way to connect to Jenkins in cluster
func (l *PipelineExecutionLifecycle) getJenkinsURL() (string, error) {

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

func (s *PipelineExecutionLifecycle) GetName() string {
	return "pipeline-execution-controller"
}
