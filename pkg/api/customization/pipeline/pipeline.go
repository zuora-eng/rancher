package pipeline

import (
	"strings"

	"fmt"
	"github.com/kubernetes/kubernetes/pkg/controller/history"
	"github.com/rancher/norman/api/access"
	"github.com/rancher/norman/api/handler"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/controllers/user/pipeline/utils"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/client/management/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"net/http"
	"time"
)

type Handler struct {
	Management config.ManagementContext
}

func Formatter(apiContext *types.APIContext, resource *types.RawResource) {
	resource.AddAction(apiContext, "activate")
	resource.AddAction(apiContext, "deactivate")
	resource.AddAction(apiContext, "run")
	resource.AddAction(apiContext, "export")
}

func CollectionFormatter(apiContext *types.APIContext, collection *types.GenericCollection) {
	//collection.Links["envvars"] = apiContext.URLBuilder.Link("envvars",collection)
}

func (h *Handler) LinkHandler(apiContext *types.APIContext, next types.RequestHandler) error {
	logrus.Debugf("enter link - %v", apiContext.Link)
	if apiContext.Link == "envvars" {
		//TODO
		return nil
	} else {
		return httperror.NewAPIError(httperror.NotFound, "Link not found")
	}
}

func (h *Handler) CreateHandler(apiContext *types.APIContext, next types.RequestHandler) error {
	//update hooks endpoint for webhook
	if err := utils.UpdateEndpoint(apiContext); err != nil {
		return err
	}
	return handler.CreateHandler(apiContext, next)
}

func (h *Handler) ActionHandler(actionName string, action *types.Action, apiContext *types.APIContext) error {
	logrus.Debugf("do pipeline action:%s", actionName)

	switch actionName {
	case "activate":
		return h.activate(apiContext)
	case "deactivate":
		return h.deactivate(apiContext)
	case "run":
		return h.run(apiContext)
	case "export":
		return h.export(apiContext)
	}
	return httperror.NewAPIError(httperror.InvalidAction, "unsupported action")
}

func (h *Handler) activate(apiContext *types.APIContext) error {
	parts := strings.Split(apiContext.ID, ":")
	ns := parts[0]
	name := parts[1]

	pipelines := h.Management.Management.Pipelines(ns)
	pipelineLister := pipelines.Controller().Lister()
	pipeline, err := pipelineLister.Get(ns, name)
	if err != nil {
		logrus.Errorf("Error while getting pipeline for %s :%v", apiContext.ID, err)
		return err
	}

	if pipeline.Status.State != "active" {
		pipeline.Status.State = "active"
		if _, err = pipelines.Update(pipeline); err != nil {
			logrus.Errorf("Error while updating pipeline:%v", err)
			return err
		}
	}

	data := map[string]interface{}{}
	if err := access.ByID(apiContext, apiContext.Version, apiContext.Type, apiContext.ID, &data); err != nil {
		return err
	}

	apiContext.WriteResponse(http.StatusOK, data)
	return nil
}

func (h *Handler) deactivate(apiContext *types.APIContext) error {
	parts := strings.Split(apiContext.ID, ":")
	ns := parts[0]
	name := parts[1]

	pipelines := h.Management.Management.Pipelines(ns)
	pipelineLister := pipelines.Controller().Lister()
	pipeline, err := pipelineLister.Get(ns, name)
	if err != nil {
		logrus.Errorf("Error while getting pipeline for %s :%v", apiContext.ID, err)
		return err
	}

	if pipeline.Status.State != "inactive" {
		pipeline.Status.State = "inactive"
		if _, err = pipelines.Update(pipeline); err != nil {
			logrus.Errorf("Error while updating pipeline:%v", err)
			return err
		}
	}

	data := map[string]interface{}{}
	if err := access.ByID(apiContext, apiContext.Version, apiContext.Type, apiContext.ID, &data); err != nil {
		return err
	}

	apiContext.WriteResponse(http.StatusOK, data)
	return nil
}

func (h *Handler) run(apiContext *types.APIContext) error {
	parts := strings.Split(apiContext.ID, ":")
	ns := parts[0]
	name := parts[1]
	pipelines := h.Management.Management.Pipelines(ns)
	pipelineLister := pipelines.Controller().Lister()

	executions := h.Management.Management.PipelineExecutions(ns)
	logs := h.Management.Management.PipelineExecutionLogs(ns)

	pipeline, err := pipelineLister.Get(ns, name)
	if err != nil {
		logrus.Errorf("Error while getting pipeline for %s :%v", apiContext.ID, err)
		return err
	}
	execution, err := utils.RunPipeline(pipelines, executions, logs, pipeline, utils.TriggerTypeUser)
	if err != nil {
		return err
	}

	data := map[string]interface{}{}
	if err := access.ByID(apiContext, apiContext.Version, client.PipelineExecutionType, ns+":"+execution.Name, &data); err != nil {
		return err
	}

	apiContext.WriteResponse(http.StatusOK, data)
	return err
}

func (h *Handler) export(apiContext *types.APIContext) error {
	return nil
}
