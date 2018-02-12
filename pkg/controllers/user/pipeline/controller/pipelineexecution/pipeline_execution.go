package pipelineexecution

import (
	"context"
	"github.com/rancher/rancher/pkg/controllers/user/pipeline/engine"
	"github.com/rancher/rancher/pkg/controllers/user/pipeline/utils"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
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
	pipelineEngine, err := engine.New(l.cluster)
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

func (s *PipelineExecutionLifecycle) GetName() string {
	return "pipeline-execution-controller"
}
