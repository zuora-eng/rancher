package pipeline

import (
	"strings"

	"fmt"
	"github.com/rancher/norman/api/access"
	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/pipeline/utils"
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

func (h *Handler) ActionHandler(actionName string, action *types.Action, apiContext *types.APIContext) error {
	logrus.Infof("do activity action:%s", actionName)
	//TODO FIXME
	//update endpoint by request url
	if err := utils.UpdateEndpoint(apiContext); err != nil {
		logrus.Errorf("update endpoint got error:%v", err)
	}

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
	return nil
}

func (h *Handler) activate(apiContext *types.APIContext) error {
	//parts := strings.Split(apiContext.ID, ":")
	//ns := parts[0]
	//id := parts[1]
	//
	//pipelineClient := h.Management.Management.Pipelines(ns)
	//pipeline, err := pipelineClient.Get(id, metav1.GetOptions{})
	//if err != nil {
	//	logrus.Errorf("Error while getting pipeline for %s :%v", apiContext.ID, err)
	//	return err
	//}
	//
	//if pipeline.Spec.Active == false {
	//	pipeline.Spec.Active = true
	//} else {
	//	return errors.New("the pipeline is already activated")
	//}
	//_, err = pipelineClient.Update(pipeline)
	//if err != nil {
	//	logrus.Errorf("Error while updating pipeline:%v", err)
	//	return err
	//}
	//
	//data := map[string]interface{}{}
	//if err := access.ByID(apiContext, apiContext.Version, apiContext.Type, apiContext.ID, &data); err != nil {
	//	return err
	//}
	//
	//apiContext.WriteResponse(http.StatusOK, data)
	return nil
}

func (h *Handler) deactivate(apiContext *types.APIContext) error {
	//parts := strings.Split(apiContext.ID, ":")
	//ns := parts[0]
	//id := parts[1]
	//
	//pipelineClient := h.Management.Management.Pipelines(ns)
	//pipeline, err := pipelineClient.Get(id, metav1.GetOptions{})
	//if err != nil {
	//	logrus.Errorf("Error while getting pipeline for %s :%v", apiContext.ID, err)
	//	return err
	//}
	//
	//if pipeline.Spec.Active == true {
	//	pipeline.Spec.Active = false
	//} else {
	//	return errors.New("the pipeline is already deactivated")
	//}
	//_, err = pipelineClient.Update(pipeline)
	//if err != nil {
	//	logrus.Errorf("Error while updating pipeline:%v", err)
	//	return err
	//}
	//
	//data := map[string]interface{}{}
	//if err := access.ByID(apiContext, apiContext.Version, apiContext.Type, apiContext.ID, &data); err != nil {
	//	return err
	//}
	//
	//apiContext.WriteResponse(http.StatusOK, data)
	return nil
}

func (h *Handler) run(apiContext *types.APIContext) error {
	parts := strings.Split(apiContext.ID, ":")
	ns := parts[0]
	id := parts[1]
	pipelineClient := h.Management.Management.Pipelines(ns)
	pipeline, err := pipelineClient.Get(id, metav1.GetOptions{})
	if err != nil {
		logrus.Errorf("Error while getting pipeline for %s :%v", apiContext.ID, err)
		return err
	}

	historyClient := h.Management.Management.PipelineExecutions(ns)
	history := utils.InitHistory(pipeline, v3.TriggerTypeManual)
	history, err = historyClient.Create(history)
	if err != nil {
		return err
	}
	//create log entries
	Logs := h.Management.Management.PipelineExecutionLogs(ns)
	for j, stage := range pipeline.Spec.Stages {
		for k, _ := range stage.Steps {
			log := &v3.PipelineExecutionLog{
				ObjectMeta: metav1.ObjectMeta{
					Name:   fmt.Sprintf("%s-%d-%d", history.Name, j, k),
					Labels: utils.PIPELINE_INPROGRESS_LABEL,
				},
				Spec: v3.PipelineExecutionLogSpec{
					ProjectName:           pipeline.Spec.ProjectName,
					PipelineExecutionName: history.Name,
					Stage: j,
					Step:  k,
				},
			}
			if _, err := Logs.Create(log); err != nil {
				return err
			}
		}
	}

	pipeline.Status.NextRun++
	pipeline.Status.LastExecutionId = history.Name
	pipeline.Status.LastStarted = time.Now().String()

	_, err = pipelineClient.Update(pipeline)

	data := map[string]interface{}{}
	if err := access.ByID(apiContext, apiContext.Version, client.PipelineExecutionType, ns+":"+history.Name, &data); err != nil {
		return err
	}

	apiContext.WriteResponse(http.StatusOK, data)
	return err
}

func (h *Handler) export(apiContext *types.APIContext) error {
	return nil
}
