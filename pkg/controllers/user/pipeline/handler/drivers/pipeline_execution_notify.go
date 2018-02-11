package drivers

import (
	"github.com/pkg/errors"
	"github.com/rancher/rancher/pkg/pipeline/utils"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const EXECUTION_NOTIFY_HEADER = "X-PipelineExecution-Notify"

type SyncExecutionDriver struct {
	Management *config.ManagementContext
}

func (s SyncExecutionDriver) Execute(req *http.Request) (int, error) {
	logrus.Debugf("get sync execution notify, header: %v \n", req.Header)
	executionId := req.FormValue("executionId")
	parts := strings.Split(executionId, ":")
	if len(parts) < 0 {
		return http.StatusUnprocessableEntity, errors.New("execution id not valid")
	}
	ns := parts[0]
	id := parts[1]
	executionClient := s.Management.Management.PipelineExecutions(ns)
	execution, err := executionClient.GetNamespaced(ns, id, metav1.GetOptions{})
	if err != nil {
		logrus.Errorf("sync execution got error: %v", err)
		return http.StatusInternalServerError, err
	}
	stagestr := req.FormValue("stage")
	stage, err := strconv.Atoi(stagestr)
	if err != nil {
		logrus.Errorf("sync execution got error: %v", err)
		return http.StatusInternalServerError, err
	}
	step, err := strconv.Atoi(req.FormValue("step"))
	if err != nil {
		logrus.Errorf("sync execution got error: %v", err)
		return http.StatusInternalServerError, err
	}
	event := req.FormValue("event")
	curTime := time.Now().String()
	if event == v3.StateBuilding {
		//TODO check
		execution.Status.Stages[stage].Steps[step].State = v3.StateBuilding
		execution.Status.Stages[stage].Steps[step].Started = curTime
		if execution.Status.Stages[stage].Started == "" {
			execution.Status.Stages[stage].State = v3.StateBuilding
			execution.Status.Stages[stage].Started = curTime
		}
		if execution.Status.Started == "" {
			execution.Status.State = v3.StateBuilding
			execution.Status.Started = curTime
		}
	} else if event == v3.StateSuccess {
		execution.Status.Stages[stage].Steps[step].State = v3.StateSuccess
		execution.Status.Stages[stage].Steps[step].Ended = curTime
		if utils.IsStageSuccess(execution.Status.Stages[stage]) {
			execution.Status.Stages[stage].State = v3.StateSuccess
			execution.Status.Stages[stage].Ended = curTime
			if stage == len(execution.Status.Stages) {
				execution.Status.State = v3.StateSuccess
				execution.Status.Ended = curTime
			}
		}
	} else if event == v3.StateFail {
		execution.Status.Stages[stage].Steps[step].State = v3.StateFail
		execution.Status.Stages[stage].Steps[step].Ended = curTime
		execution.Status.Stages[stage].State = v3.StateFail
		execution.Status.Stages[stage].Ended = curTime
		execution.Status.State = v3.StateFail
		execution.Status.Ended = curTime

	} else {
		return http.StatusInternalServerError, errors.New("unrecognized event")
	}
	_, err = executionClient.Update(execution)
	if err != nil {
		logrus.Errorf("sync execution got error: %v", err)
		return http.StatusInternalServerError, err
	}
	return http.StatusOK, nil
}
