package pipelineexecution

import (
	"context"
	"github.com/rancher/rancher/pkg/controllers/user/pipeline/engine"
	"github.com/rancher/rancher/pkg/controllers/user/pipeline/utils"
	"github.com/rancher/rancher/pkg/ticker"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	"time"
)

const (
	syncStateInterval = 20 * time.Second
)

type ExecutionStateSyncer struct {
	pipelineLister          v3.PipelineLister
	pipelines               v3.PipelineInterface
	pipelineExecutionLister v3.PipelineExecutionLister
	pipelineExecutions      v3.PipelineExecutionInterface
	cluster                 *config.UserContext
}

func (s *ExecutionStateSyncer) sync(ctx context.Context, syncInterval time.Duration) {
	for range ticker.Context(ctx, syncInterval) {
		logrus.Debugf("Start sync pipeline execution state")
		s.syncState()
		logrus.Debugf("Sync pipeline execution state complete")
	}

}

func (s *ExecutionStateSyncer) syncState() {
	executions, err := s.pipelineExecutionLister.List("", utils.PIPELINE_INPROGRESS_LABEL.AsSelector())
	if err != nil {
		logrus.Errorf("Error listing PipelineExecutions - %v", err)
	}
	if len(executions) < 1 {
		return
	}
	pipelineEngine, err := engine.New(s.cluster)
	if err != nil {
		logrus.Errorf("Error get Jenkins engine - %v", err)
	}
	for _, e := range executions {
		if e.Status.State == utils.StateWaiting || e.Status.State == utils.StateBuilding {
			updated, err := pipelineEngine.SyncExecution(e)
			if err != nil {
				logrus.Errorf("Error sync pipeline execution - %v", err)
				e.Status.State = utils.StateFail
				if _, err := s.pipelineExecutions.Update(e); err != nil {
					logrus.Errorf("Error update pipeline execution - %v", err)
				}
			} else if updated {
				if _, err := s.pipelineExecutions.Update(e); err != nil {
					logrus.Errorf("Error update pipeline execution - %v", err)
				}

				//update lastrunstate of the pipeline
				p, err := s.pipelineLister.Get(e.Spec.Pipeline.Namespace, e.Spec.Pipeline.Name)
				if err != nil {
					logrus.Errorf("Error get pipeline - %v", err)
				}
				if p.Status.LastExecutionID == e.Name {
					p.Status.LastRunState = e.Status.State
					if _, err := s.pipelines.Update(p); err != nil {
						logrus.Errorf("Error update pipeline - %v", err)
					}
				}
			}
		}
	}
}
