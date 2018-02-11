package log_syncer

import (
	"context"
	"fmt"
	"github.com/rancher/rancher/pkg/cluster/utils"
	"github.com/rancher/rancher/pkg/pipeline/engine"
	pipelineutils "github.com/rancher/rancher/pkg/pipeline/utils"
	"github.com/rancher/types/apis/core/v1"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	"time"
)

const (
	syncInterval = 20 * time.Second
)

type ExecutionLogSyncer struct {
	pipelineExecutionLister    v3.PipelineExecutionLister
	pipelineExecutionLogLister v3.PipelineExecutionLogLister
	pipelineExecutionLogs      v3.PipelineExecutionLogInterface
	nodeLister                 v1.NodeLister
	serviceLister              v1.ServiceLister
	cluster                    *config.ClusterContext
}

func Register(ctx context.Context, cluster *config.ClusterContext) {
	pipelineExecutionLister := cluster.Management.Management.PipelineExecutions("").Controller().Lister()
	pipelineExecutionLogs := cluster.Management.Management.PipelineExecutionLogs("")
	pipelineExecutionLogLister := pipelineExecutionLogs.Controller().Lister()

	nodeLister := cluster.Core.Nodes("").Controller().Lister()
	serviceLister := cluster.Core.Services("").Controller().Lister()
	s := &ExecutionLogSyncer{
		pipelineExecutionLister:    pipelineExecutionLister,
		pipelineExecutionLogLister: pipelineExecutionLogLister,
		pipelineExecutionLogs:      pipelineExecutionLogs,
		nodeLister:                 nodeLister,
		serviceLister:              serviceLister,
	}
	go s.sync(ctx, syncInterval)
}

func (s *ExecutionLogSyncer) sync(ctx context.Context, syncInterval time.Duration) {
	for range utils.TickerContext(ctx, syncInterval) {
		logrus.Debugf("Start sync pipeline execution log")
		s.syncLogs()
		logrus.Debugf("Sync pipeline execution log complete")
	}

}

func (s *ExecutionLogSyncer) syncLogs() {
	Logs, err := s.pipelineExecutionLogLister.List("", pipelineutils.PIPELINE_INPROGRESS_LABEL.AsSelector())
	if err != nil {
		logrus.Errorf("Error listing PipelineExecutionLogs - %v", err)
	}
	url, err := pipelineutils.GetJenkinsURL(s.nodeLister, s.serviceLister)
	if err != nil {
		logrus.Errorf("Error get Jenkins url - %v", err)
	}
	pipelineEngine, err := engine.New(s.cluster, url)
	if err != nil {
		logrus.Errorf("Error get Jenkins engine - %v", err)
	}
	for _, e := range Logs {
		execution, err := s.pipelineExecutionLister.Get(e.Namespace, e.Spec.PipelineExecutionName)
		if err != nil {
			logrus.Errorf("Error get pipeline execution - %v", err)
		}
		//get log if the step started
		if execution.Status.Stages[e.Spec.Stage].Steps[e.Spec.Step].State == v3.StateWaiting {
			continue
		}
		logText, err := pipelineEngine.GetStepLog(execution, e.Spec.Stage, e.Spec.Step)
		if err != nil {
			logrus.Errorf("Error get pipeline execution log - %v", err)
			e.Spec.Message += fmt.Sprintf("\nError get pipeline execution log - %v", err)
			e.Labels = pipelineutils.PIPELINE_FINISH_LABEL
			if _, err := s.pipelineExecutionLogs.Update(e); err != nil {
				logrus.Errorf("Error update pipeline execution log - %v", err)
			}
			continue
		}
		//TODO trim message
		e.Spec.Message = logText
		stepState := execution.Status.Stages[e.Spec.Stage].Steps[e.Spec.Step].State
		if stepState != v3.StateWaiting && stepState != v3.StateBuilding {
			e.Labels = pipelineutils.PIPELINE_FINISH_LABEL
		}
		if _, err := s.pipelineExecutionLogs.Update(e); err != nil {
			logrus.Errorf("Error update pipeline execution log - %v", err)
		}
	}
}
