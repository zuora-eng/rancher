package controller

import (
	"context"
	"github.com/rancher/norman/signal"
	"github.com/rancher/rancher/pkg/pipeline/controller/cron_syncer"
	"github.com/rancher/rancher/pkg/pipeline/controller/log_syncer"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Register(cluster *config.ClusterContext) {
	clusterPipelines := cluster.Management.Management.ClusterPipelines("")
	clusterPipelineSyncer := &ClusterPipelineSyncer{
		cluster: cluster,
	}
	clusterPipelines.AddHandler("cluster-pipeline-syncer", clusterPipelineSyncer.Sync)
	//clusterPipelineClient.AddLifecycle("cluster-pipeline-controller", clusterPipelineLifecycle)
	//clusterPipelineClient.AddClusterScopedHandler("cluster-pipeline-maintainer", cluster.ClusterName, clusterPipelineLifecycle.Sync)

	pipelineClient := cluster.Management.Management.Pipelines("")
	pipelineLifecycle := &PipelineLifecycle{
		cluster: cluster,
	}
	pipelineClient.AddLifecycle("pipeline-controller", pipelineLifecycle)

	pipelineHistoryClient := cluster.Management.Management.PipelineExecutions("")
	pipelineHistoryLifecycle := &PipelineHistoryLifecycle{
		cluster: cluster,
	}
	pipelineHistoryClient.AddLifecycle("pipeline-history-controller", pipelineHistoryLifecycle)

	ExecutionSyncer := &ExecutionStateSyncer{
		cluster:                 cluster,
		pipelineExecutionLister: cluster.Management.Management.PipelineExecutions("").Controller().Lister(),
	}
	pipelineClient.AddHandler("pipeline-execution-syncer", ExecutionSyncer.Sync)

	if err := initClusterPipeline(cluster); err != nil {
		logrus.Errorf("init cluster pipeline got error, %v", err)
	}
	ctx := signal.SigTermCancelContext(context.Background())
	Registerxxx(ctx, cluster)
	log_syncer.Register(ctx, cluster)
	cron_syncer.Register(ctx, cluster)
}

func initClusterPipeline(cluster *config.ClusterContext) error {
	clusterPipelines := cluster.Management.Management.ClusterPipelines("")
	clusterPipeline := &v3.ClusterPipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cluster.ClusterName,
			Namespace: cluster.ClusterName,
		},
		Spec: v3.ClusterPipelineSpec{
			ClusterName: cluster.ClusterName,
		},
	}

	if _, err := clusterPipelines.Create(clusterPipeline); err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}
	return nil
}
