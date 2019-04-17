package project

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/ghodss/yaml"
	"github.com/pkg/errors"
	"github.com/rancher/norman/api/handler"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/rancher/pkg/clustermanager"
	"github.com/rancher/rancher/pkg/monitoring"
	"github.com/rancher/rancher/pkg/ref"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	managementschema "github.com/rancher/types/apis/management.cattle.io/v3/schema"
	"github.com/rancher/types/client/management/v3"
	"github.com/rancher/types/compose"
	"github.com/rancher/types/user"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

func Formatter(apiContext *types.APIContext, resource *types.RawResource) {
	resource.AddAction(apiContext, "setpodsecuritypolicytemplate")
	resource.AddAction(apiContext, "exportYaml")
}

type Handler struct {
	Projects       v3.ProjectInterface
	ProjectLister  v3.ProjectLister
	ClusterManager *clustermanager.Manager
	UserMgr        user.Manager
}

func (h *Handler) Actions(actionName string, action *types.Action, apiContext *types.APIContext) error {
	canUpdateProject := func() bool {
		project := map[string]interface{}{
			"id": apiContext.ID,
		}

		return apiContext.AccessControl.CanDo(v3.ProjectGroupVersionKind.Group, v3.ProjectResource.Name, "update", apiContext, project, apiContext.Schema) == nil
	}

	switch actionName {
	case "setpodsecuritypolicytemplate":
		return h.setPodSecurityPolicyTemplate(actionName, action, apiContext)
	case "exportYaml":
		return h.ExportYamlHandler(actionName, action, apiContext)
	case "viewMonitoring":
		return h.viewMonitoring(actionName, action, apiContext)
	case "editMonitoring":
		if !canUpdateProject() {
			return httperror.NewAPIError(httperror.Unauthorized, "can not access")
		}
		return h.editMonitoring(actionName, action, apiContext)
	case "enableMonitoring":
		if !canUpdateProject() {
			return httperror.NewAPIError(httperror.Unauthorized, "can not access")
		}
		return h.enableMonitoring(actionName, action, apiContext)
	case "disableMonitoring":
		if !canUpdateProject() {
			return httperror.NewAPIError(httperror.Unauthorized, "can not access")
		}
		return h.disableMonitoring(actionName, action, apiContext)
	}

	return errors.Errorf("unrecognized action %v", actionName)
}

func (h *Handler) ExportYamlHandler(actionName string, action *types.Action, apiContext *types.APIContext) error {
	namespace, id := ref.Parse(apiContext.ID)
	project, err := h.ProjectLister.Get(namespace, id)
	if err != nil {
		return err
	}
	topkey := compose.Config{}
	topkey.Version = "v3"
	p := client.Project{}
	if err := convert.ToObj(project.Spec, &p); err != nil {
		return err
	}
	topkey.Projects = map[string]client.Project{}
	topkey.Projects[project.Spec.DisplayName] = p
	m, err := convert.EncodeToMap(topkey)
	if err != nil {
		return err
	}
	delete(m["projects"].(map[string]interface{})[project.Spec.DisplayName].(map[string]interface{}), "actions")
	delete(m["projects"].(map[string]interface{})[project.Spec.DisplayName].(map[string]interface{}), "links")
	data, err := json.Marshal(m)
	if err != nil {
		return err
	}

	buf, err := yaml.JSONToYAML(data)
	if err != nil {
		return err
	}
	reader := bytes.NewReader(buf)
	apiContext.Response.Header().Set("Content-Type", "text/yaml")
	http.ServeContent(apiContext.Response, apiContext.Request, "exportYaml", time.Now(), reader)
	return nil
}

func (h *Handler) viewMonitoring(actionName string, action *types.Action, apiContext *types.APIContext) error {
	namespace, id := ref.Parse(apiContext.ID)
	project, err := h.ProjectLister.Get(namespace, id)
	if err != nil {
		return httperror.WrapAPIError(err, httperror.NotFound, "none existent Project")
	}
	if project.DeletionTimestamp != nil {
		return httperror.NewAPIError(httperror.InvalidType, "deleting Project")
	}

	if !project.Spec.EnableProjectMonitoring {
		return httperror.NewAPIError(httperror.InvalidState, "disabling Monitoring")
	}

	// need to support `map[string]string` as entry value type in norman Builder.convertMap
	answers, err := convert.EncodeToMap(monitoring.GetOverwroteAppAnswers(project.Annotations))
	if err != nil {
		return httperror.WrapAPIError(err, httperror.ServerError, "failed to parse response")
	}
	apiContext.WriteResponse(http.StatusOK, map[string]interface{}{
		"answers": answers,
		"type":    "monitoringOutput",
	})
	return nil
}

func (h *Handler) editMonitoring(actionName string, action *types.Action, apiContext *types.APIContext) error {
	namespace, id := ref.Parse(apiContext.ID)
	project, err := h.ProjectLister.Get(namespace, id)
	if err != nil {
		return httperror.WrapAPIError(err, httperror.NotFound, "none existent Project")
	}
	if project.DeletionTimestamp != nil {
		return httperror.NewAPIError(httperror.InvalidType, "deleting Project")
	}

	if !project.Spec.EnableProjectMonitoring {
		return httperror.NewAPIError(httperror.InvalidState, "disabling Monitoring")
	}

	data, err := ioutil.ReadAll(apiContext.Request.Body)
	if err != nil {
		return httperror.WrapAPIError(err, httperror.InvalidBodyContent, "unable to read request content")
	}
	var input v3.MonitoringInput
	if err = json.Unmarshal(data, &input); err != nil {
		return httperror.WrapAPIError(err, httperror.InvalidBodyContent, "failed to parse request content")
	}

	project = project.DeepCopy()
	project.Annotations = monitoring.AppendAppOverwritingAnswers(project.Annotations, string(data))

	_, err = h.Projects.Update(project)
	if err != nil {
		return httperror.WrapAPIError(err, httperror.ServerError, "failed to upgrade Monitoring")
	}

	apiContext.WriteResponse(http.StatusNoContent, map[string]interface{}{})
	return nil
}

func (h *Handler) enableMonitoring(actionName string, action *types.Action, apiContext *types.APIContext) error {
	namespace, id := ref.Parse(apiContext.ID)
	project, err := h.ProjectLister.Get(namespace, id)
	if err != nil {
		return httperror.WrapAPIError(err, httperror.NotFound, "none existent Project")
	}
	if project.DeletionTimestamp != nil {
		return httperror.NewAPIError(httperror.InvalidType, "deleting Project")
	}

	if project.Spec.EnableProjectMonitoring {
		apiContext.WriteResponse(http.StatusNoContent, map[string]interface{}{})
		return nil
	}

	data, err := ioutil.ReadAll(apiContext.Request.Body)
	if err != nil {
		return httperror.WrapAPIError(err, httperror.InvalidBodyContent, "unable to read request content")
	}
	var input v3.MonitoringInput
	if err = json.Unmarshal(data, &input); err != nil {
		return httperror.WrapAPIError(err, httperror.InvalidBodyContent, "failed to parse request content")
	}

	project = project.DeepCopy()
	project.Spec.EnableProjectMonitoring = true
	project.Annotations = monitoring.AppendAppOverwritingAnswers(project.Annotations, string(data))

	_, err = h.Projects.Update(project)
	if err != nil {
		return httperror.WrapAPIError(err, httperror.ServerError, "failed to enable monitoring")
	}

	apiContext.WriteResponse(http.StatusNoContent, map[string]interface{}{})
	return nil
}

func (h *Handler) disableMonitoring(actionName string, action *types.Action, apiContext *types.APIContext) error {
	namespace, id := ref.Parse(apiContext.ID)
	project, err := h.ProjectLister.Get(namespace, id)
	if err != nil {
		return httperror.WrapAPIError(err, httperror.NotFound, "none existent Project")
	}
	if project.DeletionTimestamp != nil {
		return httperror.NewAPIError(httperror.InvalidType, "deleting Project")
	}

	if !project.Spec.EnableProjectMonitoring {
		apiContext.WriteResponse(http.StatusNoContent, map[string]interface{}{})
		return nil
	}

	project = project.DeepCopy()
	project.Spec.EnableProjectMonitoring = false

	_, err = h.Projects.Update(project)
	if err != nil {
		return httperror.WrapAPIError(err, httperror.ServerError, "failed to disable monitoring")
	}

	apiContext.WriteResponse(http.StatusNoContent, map[string]interface{}{})
	return nil
}

func (h *Handler) setPodSecurityPolicyTemplate(actionName string, action *types.Action,
	request *types.APIContext) error {
	input, err := handler.ParseAndValidateActionBody(request, request.Schemas.Schema(&managementschema.Version,
		client.SetPodSecurityPolicyTemplateInputType))
	if err != nil {
		return fmt.Errorf("error parse/validate action body: %v", err)
	}

	podSecurityPolicyTemplateName, ok :=
		input[client.PodSecurityPolicyTemplateProjectBindingFieldPodSecurityPolicyTemplateName].(string)
	if !ok && input[client.PodSecurityPolicyTemplateProjectBindingFieldPodSecurityPolicyTemplateName] != nil {
		return fmt.Errorf("could not convert: %v",
			input[client.PodSecurityPolicyTemplateProjectBindingFieldPodSecurityPolicyTemplateName])
	}

	schema := request.Schemas.Schema(&managementschema.Version, client.PodSecurityPolicyTemplateProjectBindingType)
	if schema == nil {
		return fmt.Errorf("no %v store available", client.PodSecurityPolicyTemplateProjectBindingType)
	}

	err = h.createOrUpdateBinding(request, schema, podSecurityPolicyTemplateName)
	if err != nil {
		return err
	}

	project, err := h.updateProjectPSPTID(request, podSecurityPolicyTemplateName)
	if err != nil {
		if apierrors.IsConflict(err) {
			return httperror.WrapAPIError(err, httperror.Conflict, "error updating PSPT ID")
		}
		return fmt.Errorf("error updating PSPT ID: %v", err)
	}

	request.WriteResponse(http.StatusOK, project)

	return nil
}

func (h *Handler) createOrUpdateBinding(request *types.APIContext, schema *types.Schema,
	podSecurityPolicyTemplateName string) error {
	bindings, err := schema.Store.List(request, schema, &types.QueryOptions{
		Conditions: []*types.QueryCondition{
			types.NewConditionFromString(client.PodSecurityPolicyTemplateProjectBindingFieldTargetProjectName,
				types.ModifierEQ, request.ID),
		},
	})
	if err != nil {
		return fmt.Errorf("error retrieving binding: %v", err)
	}

	if podSecurityPolicyTemplateName == "" {
		for _, binding := range bindings {
			namespace, okNamespace := binding[client.PodSecurityPolicyTemplateProjectBindingFieldNamespaceId].(string)
			name, okName := binding[client.PodSecurityPolicyTemplateProjectBindingFieldName].(string)

			if okNamespace && okName {
				_, err := schema.Store.Delete(request, schema, namespace+":"+name)
				if err != nil {
					return fmt.Errorf("error deleting binding: %v", err)
				}
			} else {
				return fmt.Errorf("could not convert name or namespace field: %v %v",
					binding[client.PodSecurityPolicyTemplateProjectBindingFieldNamespaceId],
					binding[client.PodSecurityPolicyTemplateProjectBindingFieldName])
			}
		}
	} else {
		if len(bindings) == 0 {
			err = h.createNewBinding(request, schema, podSecurityPolicyTemplateName)
			if err != nil {
				return fmt.Errorf("error creating binding: %v", err)
			}
		} else {
			binding := bindings[0]

			id, ok := binding["id"].(string)
			if ok {
				split := strings.Split(id, ":")

				binding, err = schema.Store.ByID(request, schema, split[0]+":"+split[len(split)-1])
				if err != nil {
					return fmt.Errorf("error retreiving binding: %v for %v", err, bindings[0])
				}
				err = h.updateBinding(binding, request, schema, podSecurityPolicyTemplateName)
				if err != nil {
					return err
				}
			} else {
				return fmt.Errorf("could not convert id field: %v", binding["id"])
			}
		}
	}

	return nil
}

func (h *Handler) updateProjectPSPTID(request *types.APIContext,
	podSecurityPolicyTemplateName string) (*v3.Project, error) {

	split := strings.Split(request.ID, ":")
	project, err := h.ProjectLister.Get(split[0], split[len(split)-1])
	if err != nil {
		return nil, fmt.Errorf("error getting project: %v", err)
	}
	project = project.DeepCopy()
	project.Status.PodSecurityPolicyTemplateName = podSecurityPolicyTemplateName

	return h.Projects.Update(project)
}

func (h *Handler) createNewBinding(request *types.APIContext, schema *types.Schema,
	podSecurityPolicyTemplateName string) error {
	binding := make(map[string]interface{})
	binding["targetProjectId"] = request.ID
	binding["podSecurityPolicyTemplateId"] = podSecurityPolicyTemplateName
	binding["namespaceId"] = strings.Split(request.ID, ":")[0]

	_, err := schema.Store.Create(request, schema, binding)
	return err
}

func (h *Handler) updateBinding(binding map[string]interface{}, request *types.APIContext, schema *types.Schema,
	podSecurityPolicyTemplateName string) error {
	binding[client.PodSecurityPolicyTemplateProjectBindingFieldPodSecurityPolicyTemplateName] =
		podSecurityPolicyTemplateName
	id, err := getID(binding["id"])
	if err != nil {
		return err
	}
	binding["id"] = id

	if _, ok := binding["id"].(string); ok && id != "" {
		var err error
		binding, err = schema.Store.Update(request, schema, binding, id)
		if err != nil {
			return fmt.Errorf("error updating binding: %v", err)
		}
	} else {
		return fmt.Errorf("could not parse: %v", binding["id"])
	}

	return nil
}

func getID(id interface{}) (string, error) {
	s, ok := id.(string)
	if !ok {
		return "", fmt.Errorf("could not convert %v", id)
	}

	split := strings.Split(s, ":")
	return split[0] + ":" + split[len(split)-1], nil
}
