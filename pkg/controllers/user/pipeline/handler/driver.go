package handler

import (
	"github.com/rancher/rancher/pkg/controllers/user/pipeline/handler/drivers"
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
}
