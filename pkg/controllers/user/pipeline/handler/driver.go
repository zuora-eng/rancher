package handler

import (
	"github.com/rancher/rancher/pkg/pipeline/handler/drivers"
	"github.com/rancher/types/config"
	"net/http"
)

var Drivers map[string]Driver

type Driver interface {
	Execute(req *http.Request) (int, error)
}

func RegisterDrivers(Management *config.ManagementContext) {
	Drivers = map[string]Driver{}
	Drivers[drivers.GITHUB_WEBHOOK_HEADER] = drivers.GithubDriver{Management: Management}
	Drivers[drivers.EXECUTION_NOTIFY_HEADER] = drivers.SyncExecutionDriver{Management: Management}
}
