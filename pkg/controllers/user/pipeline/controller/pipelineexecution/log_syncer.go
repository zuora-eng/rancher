package pipelineexecution

import (
	"context"
	"fmt"
	"github.com/rancher/rancher/pkg/controllers/user/pipeline/engine"
	"github.com/rancher/rancher/pkg/controllers/user/pipeline/utils"
	"github.com/rancher/rancher/pkg/ticker"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	"time"
)

const (
	syncLogInterval = 20 * time.Second
)

type ExecutionLogSyncer struct {
	pipelineExecutionLister    v3.PipelineExecutionLister
	pipelineExecutionLogLister v3.PipelineExecutionLogLister
	pipelineExecutionLogs      v3.PipelineExecutionLogInterface
	cluster                    *config.UserContext
}

func (s *ExecutionLogSyncer) sync(ctx context.Context, syncInterval time.Duration) {
	for range ticker.Context(ctx, syncInterval) {
		logrus.Debugf("Start sync pipeline execution log")
		s.syncLogs()
		logrus.Debugf("Sync pipeline execution log complete")
	}

}

func (s *ExecutionLogSyncer) syncLogs() {
	Logs, err := s.pipelineExecutionLogLister.List("", utils.PIPELINE_INPROGRESS_LABEL.AsSelector())
	if err != nil {
		logrus.Errorf("Error listing PipelineExecutionLogs - %v", err)
	}
	if len(Logs) < 1 {
		return
	}
	pipelineEngine, err := engine.New(s.cluster)
	if err != nil {
		logrus.Errorf("Error get Jenkins engine - %v", err)
	}
	for _, e := range Logs {
		execution, err := s.pipelineExecutionLister.Get(e.Namespace, e.Spec.PipelineExecutionName)
		if err != nil {
			logrus.Errorf("Error get pipeline execution - %v", err)
		}
		//get log if the step started
		if execution.Status.Stages[e.Spec.Stage].Steps[e.Spec.Step].State == utils.StateWaiting {
			continue
		}
		logText, err := pipelineEngine.GetStepLog(execution, e.Spec.Stage, e.Spec.Step)
		if err != nil {
			logrus.Errorf("Error get pipeline execution log - %v", err)
			e.Spec.Message += fmt.Sprintf("\nError get pipeline execution log - %v", err)
			e.Labels = utils.PIPELINE_FINISH_LABEL
			if _, err := s.pipelineExecutionLogs.Update(e); err != nil {
				logrus.Errorf("Error update pipeline execution log - %v", err)
			}
			continue
		}
		//TODO trim message
		e.Spec.Message = logText
		stepState := execution.Status.Stages[e.Spec.Stage].Steps[e.Spec.Step].State
		if stepState != utils.StateWaiting && stepState != utils.StateBuilding {
			e.Labels = utils.PIPELINE_FINISH_LABEL
		}
		if _, err := s.pipelineExecutionLogs.Update(e); err != nil {
			logrus.Errorf("Error update pipeline execution log - %v", err)
		}
	}
}
