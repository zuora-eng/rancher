package clusterpipeline

import (
	"github.com/pkg/errors"
	"github.com/rancher/rancher/pkg/controllers/user/pipeline/utils"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ClusterPipelineSyncer struct {
	cluster *config.UserContext
}

func Register(cluster *config.UserContext) {
	clusterPipelines := cluster.Management.Management.ClusterPipelines("")
	clusterPipelineSyncer := &ClusterPipelineSyncer{
		cluster: cluster,
	}
	clusterPipelines.AddHandler("cluster-pipeline-syncer", clusterPipelineSyncer.Sync)
}

func (c *ClusterPipelineSyncer) Sync(key string, obj *v3.ClusterPipeline) error {
	//ensure clusterpipeline singleton in the cluster
	utils.InitClusterPipeline(c.cluster)

	if obj.Spec.Deploy {
		if err := c.deploy(); err != nil {
			return err
		}
	} else {
		if err := c.destroy(); err != nil {
			return err
		}
	}

	return nil
}

func (c *ClusterPipelineSyncer) destroy() error {
	if err := c.cluster.Core.Namespaces("").Delete("cattle-pipeline", &metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {

		logrus.Errorf("Error occured while removing ns: %v", err)
		return err

	}

	return nil
}

func (c *ClusterPipelineSyncer) deploy() error {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cattle-pipeline",
		},
	}
	if _, err := c.cluster.Core.Namespaces("").Create(ns); err != nil && !apierrors.IsAlreadyExists(err) {
		logrus.Errorf("Error occured while create ns: %v", err)
		return errors.Wrapf(err, "Creating ns")
	}

	secret := getSecret()
	if _, err := c.cluster.Core.Secrets("").Create(secret); err != nil && !apierrors.IsAlreadyExists(err) {
		logrus.Errorf("Error occured while create secret: %v", err)
		return errors.Wrapf(err, "Creating secret")
	}

	configmap := getConfigMap()
	if _, err := c.cluster.K8sClient.CoreV1().ConfigMaps("cattle-pipeline").Create(configmap); err != nil && !apierrors.IsAlreadyExists(err) {
		logrus.Errorf("Error occured while create configmap: %v", err)
		return errors.Wrapf(err, "Creating configmap")
	}

	service := getJenkinsService()
	if _, err := c.cluster.Core.Services("").Create(service); err != nil && !apierrors.IsAlreadyExists(err) {
		logrus.Errorf("Error occured while create service: %v", err)
		return errors.Wrapf(err, "Creating service")
	}

	agentservice := getJenkinsAgentService()
	if _, err := c.cluster.Core.Services("").Create(agentservice); err != nil && !apierrors.IsAlreadyExists(err) {
		logrus.Errorf("Error occured while create service: %v", err)
		return errors.Wrapf(err, "Creating service")
	}

	sa := getServiceAccount()
	if _, err := c.cluster.K8sClient.CoreV1().ServiceAccounts("cattle-pipeline").Create(sa); err != nil && !apierrors.IsAlreadyExists(err) {
		logrus.Errorf("Error occured while create service account: %v", err)
		return errors.Wrapf(err, "Creating service account")
	}
	rb := getRoleBindings()
	if _, err := c.cluster.K8sClient.RbacV1().ClusterRoleBindings().Create(rb); err != nil && !apierrors.IsAlreadyExists(err) {
		logrus.Errorf("Error occured while create role binding: %v", err)
		return errors.Wrapf(err, "Creating role binding")
	}
	deployment := getJenkinsDeployment()
	if _, err := c.cluster.Apps.Deployments("").Create(deployment); err != nil && !apierrors.IsAlreadyExists(err) {
		logrus.Errorf("Error occured while create deployment: %v", err)
		return errors.Wrapf(err, "Creating deployment")
	}

	return nil
}
