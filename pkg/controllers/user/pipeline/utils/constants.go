package utils

const (
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
