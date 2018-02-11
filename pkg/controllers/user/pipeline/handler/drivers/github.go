package drivers

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/google/go-github/github"
	"github.com/pkg/errors"
	"github.com/rancher/rancher/pkg/pipeline/utils"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	"io/ioutil"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"net/http"
	"strings"
	"time"
)

const GITHUB_WEBHOOK_HEADER = "X-GitHub-Event"

type GithubDriver struct {
	Management *config.ManagementContext
}

func (g GithubDriver) Execute(req *http.Request) (int, error) {
	var signature string
	if signature = req.Header.Get("X-Hub-Signature"); len(signature) == 0 {
		logrus.Errorf("receive github webhook,no signature")
		return http.StatusUnprocessableEntity, errors.New("github webhook missing signature")
	}
	event := req.Header.Get(GITHUB_WEBHOOK_HEADER)
	if event == "ping" {
		return http.StatusOK, nil
	} else if event != "push" {
		//event != "pull_request"
		return http.StatusUnprocessableEntity, fmt.Errorf("not trigger for event:%s", event)
	}

	pipelineId := req.FormValue("pipelineId")
	parts := strings.Split(pipelineId, ":")
	if len(parts) < 0 {
		return http.StatusUnprocessableEntity, errors.New("pipeline id not valid")
	}
	ns := parts[0]
	id := parts[1]
	pipelineClient := g.Management.Management.Pipelines(ns)
	pipeline, err := pipelineClient.GetNamespaced(ns, id, metav1.GetOptions{})
	if err != nil {
		return http.StatusInternalServerError, err
	}

	//////
	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		logrus.Errorf("receive github webhook, got error:%v", err)
		return http.StatusUnprocessableEntity, err
	}
	if match := VerifyGithubWebhookSignature([]byte(pipeline.Status.Token), signature, body); !match {
		logrus.Errorf("receive github webhook, invalid signature")
		return http.StatusUnprocessableEntity, errors.New("github webhook invalid signature")
	}
	//check branch
	payload := &github.WebHookPayload{}
	if err := json.Unmarshal(body, payload); err != nil {
		logrus.Error("fail to parse github webhook payload")
		return http.StatusUnprocessableEntity, err
	}
	//TODO
	if *payload.Ref != "refs/heads/"+pipeline.Spec.Stages[0].Steps[0].SourceCodeConfig.Branch {
		logrus.Warningf("branch not match:%v,%v", *payload.Ref, pipeline.Spec.Stages[0].Steps[0].SourceCodeConfig.Branch)
		return http.StatusInternalServerError, errors.New("branch not match")
	}

	//Generate a new pipeline execution
	historyClient := g.Management.Management.PipelineExecutions(ns)
	history := utils.InitHistory(pipeline, v3.TriggerTypeWebhook)
	history, err = historyClient.Create(history)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	pipeline.Status.NextRun++
	pipeline.Status.LastExecutionId = history.Name
	pipeline.Status.LastStarted = time.Now().String()

	_, err = pipelineClient.Update(pipeline)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	return http.StatusOK, nil
}

func VerifyGithubWebhookSignature(secret []byte, signature string, body []byte) bool {

	const signaturePrefix = "sha1="
	const signatureLength = 45 // len(SignaturePrefix) + len(hex(sha1))

	if len(signature) != signatureLength || !strings.HasPrefix(signature, signaturePrefix) {
		return false
	}

	actual := make([]byte, 20)
	hex.Decode(actual, []byte(signature[5:]))
	computed := hmac.New(sha1.New, secret)
	computed.Write(body)

	return hmac.Equal([]byte(computed.Sum(nil)), actual)
}

func VerifyGitRefs(pipeline v3.Pipeline, refs string) {
	payload := &github.WebHookPayload{}
	payload.GetRef()
	if refs == "refs/heads/"+pipeline.Spec.Stages[0].Steps[0].SourceCodeConfig.Branch {

	}
}
