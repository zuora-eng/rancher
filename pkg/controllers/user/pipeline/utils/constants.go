package utils

const (
	PIPELINE_NAMESPACE = "cattle-pipeline"
	DEFAULT_REGISTRY   = "index.docker.io"
	DEFAULT_TAG        = "latest"

	StepTypeSourceCode   = "sourceCode"
	StepTypeRunScript    = "runScript"
	StepTypePublishImage = "publishImage"
	TriggerTypeCron      = "cron"
	TriggerTypeUser      = "user"
	TriggerTypeWebhook   = "webhook"

	StateWaiting  = "Waiting"
	StateBuilding = "Building"
	StateSuccess  = "Success"
	StateFail     = "Fail"
	StateError    = "Error"
	StateSkip     = "Skipped"
	StateAbort    = "Abort"
	StatePending  = "Pending"
	StateDenied   = "Denied"
)

var PRESERVERED_ENV_VARS = [...]string{"CICD_GIT_COMMIT", "CICD_GIT_BRANCH", "CICD_GIT_COMMITTER_NAME",
	"CICD_GIT_URL", "CICD_GIT_REPOSITORY", "CICD_PIPELINE_NAME", "CICD_PIPELINE_ID",
	"CICD_TRIGGER_TYPE", "CICD_EXECUTION_ID",
	"CICD_EXECUTION_NUMBER",
}
