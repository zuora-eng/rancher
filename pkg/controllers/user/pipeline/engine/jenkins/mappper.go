package jenkins

import (
	"bytes"
	"fmt"
	"github.com/rancher/rancher/pkg/controllers/user/pipeline/utils"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"regexp"
	"strings"
)

func ConvertPipelineToJenkinsPipeline(pipeline *v3.Pipeline) PipelineJob {
	pipelineJob := PipelineJob{
		Plugin: WORKFLOW_JOB_PLUGIN,
		Definition: Definition{
			Class:   FLOW_DEFINITION_CLASS,
			Plugin:  FLOW_DEFINITION_PLUGIN,
			Sandbox: true,
			Script:  convertPipeline(pipeline),
		},
	}

	return pipelineJob
}

func convertStep(pipeline *v3.Pipeline, stageOrdinal int, stepOrdinal int) string {

	stepContent := ""
	stepName := fmt.Sprintf("step-%d-%d", stageOrdinal, stepOrdinal)
	step := pipeline.Spec.Stages[stageOrdinal].Steps[stepOrdinal]

	if step.SourceCodeConfig != nil {
		branch := step.SourceCodeConfig.Branch
		branchCondition := step.SourceCodeConfig.BranchCondition
		//default only branch xxx
		if branchCondition == "except" {
			branch = fmt.Sprintf(":^(?!(%s))", branch)
		} else if branchCondition == "all" {
			branch = "**"
		}
		stepContent = fmt.Sprintf("git url: '%s', branch: '%s', credentialsId: '%s'", step.SourceCodeConfig.URL, branch, step.SourceCodeConfig.SourceCodeCredentialName)
	} else if step.RunScriptConfig != nil {
		stepContent = fmt.Sprintf(`sh """%s"""`, step.RunScriptConfig.ShellScript)
	} else if step.PublishImageConfig != nil {
		stepContent = fmt.Sprintf(`sh """/usr/local/bin/dockerd-entrypoint.sh /bin/drone-docker"""`)
	} else {
		return ""
	}
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
			options := ""
			if step.SourceCodeConfig != nil {
				image = "alpine/git"
			} else if step.RunScriptConfig != nil {
				image = step.RunScriptConfig.Image
			} else if step.PublishImageConfig != nil {
				registry, repo, tag := utils.SplitImageTag(step.PublishImageConfig.Tag)
				//TODO key-key mapping instead of registry-key mapping
				reg, _ := regexp.Compile("[^a-zA-Z0-9]+")
				proceccedRegistry := strings.ToLower(reg.ReplaceAllString(registry, ""))
				secretName := fmt.Sprintf("%s-%s", pipeline.Namespace, proceccedRegistry)
				image = "plugins/docker"
				publishoption := `, privileged: true, envVars: [
			envVar(key: 'PLUGIN_REPO', value: '%s/%s'),
			envVar(key: 'PLUGIN_TAG', value: '%s'),
			envVar(key: 'PLUGIN_DOCKERFILE', value: '%s'),
			envVar(key: 'PLUGIN_CONTEXT', value: '%s'),
			envVar(key: 'DOCKER_REGISTRY', value: '%s'),
            secretEnvVar(key: 'DOCKER_USERNAME', secretName: '%s', secretKey: 'username'),
            secretEnvVar(key: 'DOCKER_PASSWORD', secretName: '%s', secretKey: 'password'),
        ]`
				options = fmt.Sprintf(publishoption, registry, repo, tag, step.PublishImageConfig.DockerfilePath, step.PublishImageConfig.BuildContext, registry, secretName, secretName)
			} else {
				return ""
			}
			containerDef := fmt.Sprintf(containerBlock, stepName, image, options)
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

const containerBlock = `containerTemplate(name: '%s', image: '%s', ttyEnabled: true, command: 'cat' %s),`
