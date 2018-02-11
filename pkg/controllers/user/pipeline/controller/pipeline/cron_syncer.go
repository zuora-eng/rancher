package cron_syncer

import (
	"context"
	"github.com/pkg/errors"
	"github.com/rancher/rancher/pkg/cluster/utils"
	pipelineutils "github.com/rancher/rancher/pkg/pipeline/utils"
	"github.com/rancher/types/apis/core/v1"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/robfig/cron"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/labels"
	"time"
)

const (
	syncInterval = 60 * time.Second
)

type CronSyncer struct {
	pipelineLister          v3.PipelineLister
	pipelines               v3.PipelineInterface
	pipelineExecutions      v3.PipelineExecutionInterface
	pipelineExecutionLister v3.PipelineExecutionLister
	nodeLister              v1.NodeLister
	serviceLister           v1.ServiceLister
}

func Register(ctx context.Context, cluster *config.ClusterContext) {
	pipelines := cluster.Management.Management.Pipelines("")
	pipelineLister := pipelines.Controller().Lister()
	pipelineExecutions := cluster.Management.Management.PipelineExecutions("")
	pipelineExecutionLister := cluster.Management.Management.PipelineExecutions("").Controller().Lister()

	nodeLister := cluster.Core.Nodes("").Controller().Lister()
	serviceLister := cluster.Core.Services("").Controller().Lister()
	s := &CronSyncer{
		pipelineLister:          pipelineLister,
		pipelines:               pipelines,
		pipelineExecutionLister: pipelineExecutionLister,
		pipelineExecutions:      pipelineExecutions,
		nodeLister:              nodeLister,
		serviceLister:           serviceLister,
	}
	go s.sync(ctx, syncInterval)
}

func (s *CronSyncer) sync(ctx context.Context, syncInterval time.Duration) {
	for range utils.TickerContext(ctx, syncInterval) {
		logrus.Debugf("Start sync pipeline execution log")
		s.syncCron()
		logrus.Debugf("Sync pipeline execution log complete")
	}
}

func (s *CronSyncer) syncCron() {
	pipelines, err := s.pipelineLister.List("", labels.NewSelector())
	if err != nil {
		logrus.Errorf("Error listing pipelines")
		return
	}
	for _, p := range pipelines {
		s.checkCron(p)
	}
}

func (s *CronSyncer) checkCron(pipeline *v3.Pipeline) {
	if pipeline.Spec.TriggerCronExpression == "" {
		return
	}
	if pipeline.Status.NextStart == "" {
		//update nextstart time
		nextStart, err := getNextStartTime(pipeline.Spec.TriggerCronExpression, pipeline.Spec.TriggerCronTimezone, time.Now())
		if err != nil {
			logrus.Errorf("Error getNextStartTime - %v", err)
			return
		}
		pipeline.Status.NextStart = nextStart
		if _, err := s.pipelines.Update(pipeline); err != nil {
			logrus.Errorf("Error update pipeline - %v", err)
		}
		return
	}

	nextStartTime, err := time.ParseInLocation(time.RFC3339, pipeline.Status.NextStart, time.Local)
	if err != nil {
		logrus.Errorf("Error parsing nextStart - %v", err)
		s.resetNextRun(pipeline)
		return
	}
	if nextStartTime.After(time.Now()) {
		//not time up
		return
	} else if nextStartTime.Before(time.Now()) && nextStartTime.Add(syncInterval).After(time.Now()) {
		//trigger run
		nextStart, err := getNextStartTime(pipeline.Spec.TriggerCronExpression, pipeline.Spec.TriggerCronTimezone, time.Now())
		if err != nil {
			logrus.Errorf("Error getNextStartTime - %v", err)
			return
		}
		pipeline.Status.NextStart = nextStart
		if err := pipelineutils.RunPipeline(s.pipelines, s.pipelineExecutions, pipeline, v3.TriggerTypeCron); err != nil {
			logrus.Errorf("Error run pipeline - %v", err)
			return
		}
	} else {
		//stale nextStart
		s.resetNextRun(pipeline)
	}

}

func getNextStartTime(cronExpression string, timezone string, fromTime time.Time) (string, error) {
	//use Local as default
	loc, err := time.LoadLocation(timezone)
	if err != nil || timezone == "" || timezone == "Local" {
		loc = time.Local
		if err != nil {
			logrus.Errorf("Failed to load time zone %v: %+v,use local timezone instead", timezone, err)
		}
	}
	if cronExpression == "* * * * *" {
		return "", errors.New("'* * * * *' for cron is not allowed and ignored")
	}
	schedule, err := cron.ParseStandard(cronExpression)
	if err != nil {
		return "", err
	}

	return schedule.Next(fromTime.In(loc)).Format(time.RFC3339), nil
}

func (s *CronSyncer) resetNextRun(pipeline *v3.Pipeline) {
	pipeline.Status.NextStart = ""
	if _, err := s.pipelines.Update(pipeline); err != nil {
		logrus.Errorf("Error update pipeline - %v", err)
	}
}
