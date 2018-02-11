package jenkins

import (
	"bytes"
	"fmt"
	"github.com/rancher/rancher/pkg/pipeline/utils"
	"github.com/rancher/types/apis/management.cattle.io/v3"
)

func ConvertPipelineToJenkinsPipeline(pipeline *v3.Pipeline) PipelineJob {
	pipelineJob := PipelineJob{
		Plugin: WORKFLOW_JOB_PLUGIN,
		Definition: Definition{
			Class:   FLOW_DEFINITION_CLASS,
			Plugin:  FLOW_DEFINITION_PLUGIN,
			Sandbox: true,
			//TODO test script
			Script: convertPipeline(pipeline),
		},
	}

	return pipelineJob
}

func convertStep(pipeline *v3.Pipeline, stageOrdinal int, stepOrdinal int) string {
	//var buffer bytes.Buffer
	stepContent := ""
	stepName := fmt.Sprintf("step-%d-%d", stageOrdinal, stepOrdinal)
	step := pipeline.Spec.Stages[stageOrdinal].Steps[stepOrdinal]

	//buffer.WriteString(preStepScript(pipeline, stageOrdinal, stepOrdinal))
	//buffer.WriteString("\n")
	if step.SourceCodeConfig != nil {
		branch := step.SourceCodeConfig.Branch
		branchCondition := step.SourceCodeConfig.BranchCondition
		//default only branch xxx
		if branchCondition == "except" {
			branch = fmt.Sprintf(":^(?!(%s))", branch)
		} else if branchCondition == "all" {
			branch = "*"
		}
		stepContent = fmt.Sprintf("git url: '%s', branch: '%s', credentialsId: '%s'", step.SourceCodeConfig.Url, branch, step.SourceCodeConfig.SourceCodeCredentialName)
	} else if step.RunScriptConfig != nil {
		stepContent = fmt.Sprintf(`sh """%s"""`, step.RunScriptConfig.ShellScript)
	} else if step.PublishImageConfig != nil {
		stepContent = fmt.Sprintf(`sh """"echo dopublishimage"""`)
	} else {
		return ""
	}
	//buffer.WriteString(stepContent)
	//buffer.WriteString("\n")
	//buffer.WriteString(postStepScript(pipeline, stageOrdinal, stepOrdinal))

	return fmt.Sprintf(stepBlock, stepName, stepName, stepName, stepContent)
}

func convertStage(pipeline *v3.Pipeline, stageOrdinal int) string {
	var buffer bytes.Buffer
	stage := pipeline.Spec.Stages[stageOrdinal]
	for i, _ := range stage.Steps {
		buffer.WriteString(convertStep(pipeline, stageOrdinal, i))
		if i != len(stage.Steps)-1 {
			buffer.WriteString(",")
		}
	}

	return fmt.Sprintf(stageBlock, stage.Name, buffer.String())
}

func convertPipeline(pipeline *v3.Pipeline) string {
	var containerbuffer bytes.Buffer
	var pipelinebuffer bytes.Buffer
	for j, stage := range pipeline.Spec.Stages {
		pipelinebuffer.WriteString(convertStage(pipeline, j))
		pipelinebuffer.WriteString("\n")
		for k, step := range stage.Steps {
			stepName := fmt.Sprintf("step-%d-%d", j, k)
			image := ""
			if step.SourceCodeConfig != nil {
				image = "alpine/git"
			} else if step.RunScriptConfig != nil {
				image = step.RunScriptConfig.Image
			} else if step.PublishImageConfig != nil {
				image = "docker"
			} else {
				return ""
			}
			containerDef := fmt.Sprintf(containerBlock, stepName, image)
			containerbuffer.WriteString(containerDef)

		}
	}

	return fmt.Sprintf(pipelineBlock, containerbuffer.String(), pipelinebuffer.String())
}

const stageBlock = `stage('%s'){
parallel %s
}
`

const stepBlock = `'%s': {
  stage('%s'){
    container(name: '%s') {
      %s
    }
  }
}
`

const pipelineBlock = `def label = "buildpod.${env.JOB_NAME}.${env.BUILD_NUMBER}".replace('-', '_').replace('/', '_')
podTemplate(label: label, containers: [
%s
containerTemplate(name: 'jnlp', image: 'jenkinsci/jnlp-slave:alpine', envVars: [
envVar(key: 'JENKINS_URL', value: 'http://jenkins:8080')], args: '${computer.jnlpmac} ${computer.name}', ttyEnabled: false)]) {
node(label) {
timestamps {
%s
}
}
}`

const containerBlock = `containerTemplate(name: '%s', image: '%s', ttyEnabled: true, command: 'cat'),`

func preStepScript(pipeline *v3.Pipeline, stage int, step int) string {
	if pipeline == nil {
		return ""
	}
	executionId := fmt.Sprintf("%s-%d", pipeline.Name, pipeline.Status.NextRun)
	skel := `sh 'curl -k -d "executionId=%s&stage=%d&step=%d&event=%s&token=%s" -H X-PipelineExecution-Notify:1 %s'`
	script := fmt.Sprintf(skel, executionId, stage, step, v3.StateBuilding, pipeline.Status.Token, utils.CI_ENDPOINT)
	return script
}

func postStepScript(pipeline *v3.Pipeline, stage int, step int) string {
	if pipeline == nil {
		return ""
	}
	executionId := fmt.Sprintf("%s-%d", pipeline.Name, pipeline.Status.NextRun)
	skel := `sh 'curl -k -d "executionId=%s&stage=%d&step=%d&event=%s&token=%s" -H X-PipelineExecution-Notify:1 %s'`
	successScript := fmt.Sprintf(skel, executionId, stage, step, v3.StateSuccess, pipeline.Status.Token, utils.CI_ENDPOINT)
	failScript := fmt.Sprintf(skel, executionId, stage, step, v3.StateFail, pipeline.Status.Token, utils.CI_ENDPOINT)
	//abortScript := fmt.Sprintf(skel, drivers.EXECUTION_NOTIFY_HEADER, executionId, stage, step, v3.StateAbort, pipeline.Status.Token, pipeline2.CI_ENDPOINT)

	postSkel := `post {
		success {
			%s
        }
        failure {
  			%s
        }
	}`
	postScript := fmt.Sprintf(postSkel, successScript, failScript)
	return postScript
}
