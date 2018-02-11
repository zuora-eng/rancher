package pipeline

import (
	"fmt"
	"github.com/pkg/errors"
	"github.com/rancher/norman/api/access"
	"github.com/rancher/norman/types"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/client/management/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"net/http"
	"strconv"
	"strings"
)

type ExecutionHandler struct {
	Management config.ManagementContext
}

func (h ExecutionHandler) ExecutionFormatter(apiContext *types.APIContext, resource *types.RawResource) {
	resource.AddAction(apiContext, "rerun")
	resource.AddAction(apiContext, "stop")
	resource.Links["log"] = apiContext.URLBuilder.Link("log", resource)

}

func (h ExecutionHandler) LinkHandler(apiContext *types.APIContext) error {
	logrus.Debugf("enter link - %v", apiContext.Link)
	if apiContext.Link == "log" {
		return h.log(apiContext)
	}
	return nil
}

func (h *ExecutionHandler) ActionHandler(actionName string, action *types.Action, apiContext *types.APIContext) error {
	logrus.Infof("do activity action:%s", actionName)

	switch actionName {
	case "rerun":
		return h.rerun(apiContext)

	case "deactivate":
		return h.stop(apiContext)
	case "notify":
		return h.notify(apiContext)
	}
	return nil
}

func (h *ExecutionHandler) rerun(apiContext *types.APIContext) error {
	return nil
}

func (h *ExecutionHandler) stop(apiContext *types.APIContext) error {
	return nil
}

func (h *ExecutionHandler) notify(apiContext *types.APIContext) error {
	stepName := apiContext.Request.FormValue("stepName")
	state := apiContext.Request.FormValue("state")
	//TODO token check
	//token := apiContext.Request.FormValue("token")
	parts := strings.Split(stepName, "-")
	if len(parts) < 3 {
		return errors.New("invalid stepName")
	}
	stageOrdinal, err := strconv.Atoi(parts[1])
	if err != nil {
		return err
	}
	stepOrdinal, err := strconv.Atoi(parts[2])
	if err != nil {
		return err
	}
	parts = strings.Split(apiContext.ID, ":")
	ns := parts[0]
	id := parts[1]
	pipelineHistoryClient := h.Management.Management.PipelineExecutions(ns)
	pipelineHistory, err := pipelineHistoryClient.Get(id, metav1.GetOptions{})
	if err != nil {
		return err
	}
	if len(pipelineHistory.Status.Stages) < stageOrdinal ||
		len(pipelineHistory.Status.Stages[stageOrdinal].Steps) < stepOrdinal {
		return errors.New("invalid status")
	}
	if state == "start" {
		pipelineHistory.Status.Stages[stageOrdinal].Steps[stepOrdinal].State = v3.StateBuilding
	} else if state == "success" {
		pipelineHistory.Status.Stages[stageOrdinal].Steps[stepOrdinal].State = v3.StateSuccess
	} else if state == "fail" {
		pipelineHistory.Status.Stages[stageOrdinal].Steps[stepOrdinal].State = v3.StateFail
	} else {
		return errors.New("unknown state")
	}
	if _, err := pipelineHistoryClient.Update(pipelineHistory); err != nil {
		return err
	}

	return nil
}

func (h *ExecutionHandler) log(apiContext *types.APIContext) error {
	stage := apiContext.Request.URL.Query().Get("stage")
	step := apiContext.Request.URL.Query().Get("step")
	if stage == "" || step == "" {
		return errors.New("Step index for log is not provided")
	}

	logId := fmt.Sprintf("%s-%s-%s", apiContext.ID, stage, step)
	data := map[string]interface{}{}
	if err := access.ByID(apiContext, apiContext.Version, client.PipelineExecutionLogType, logId, &data); err != nil {
		return err
	}

	apiContext.WriteResponse(http.StatusOK, data)
	return nil
}
