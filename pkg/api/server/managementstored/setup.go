package managementstored

import (
	"context"
	"github.com/rancher/rancher/pkg/namespace"
	"net/http"

	"github.com/rancher/norman/store/crd"
	"github.com/rancher/norman/store/proxy"
	"github.com/rancher/norman/store/subtype"
	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/api/customization/alert"
	"github.com/rancher/rancher/pkg/api/customization/app"
	"github.com/rancher/rancher/pkg/api/customization/authn"
	"github.com/rancher/rancher/pkg/api/customization/catalog"
	ccluster "github.com/rancher/rancher/pkg/api/customization/cluster"
	"github.com/rancher/rancher/pkg/api/customization/clusterregistrationtokens"
	"github.com/rancher/rancher/pkg/api/customization/cred"
	"github.com/rancher/rancher/pkg/api/customization/globaldns"
	"github.com/rancher/rancher/pkg/api/customization/globalresource"
	"github.com/rancher/rancher/pkg/api/customization/kontainerdriver"
	"github.com/rancher/rancher/pkg/api/customization/logging"
	"github.com/rancher/rancher/pkg/api/customization/monitor"
	"github.com/rancher/rancher/pkg/api/customization/multiclusterapp"
	"github.com/rancher/rancher/pkg/api/customization/node"
	"github.com/rancher/rancher/pkg/api/customization/nodetemplate"
	"github.com/rancher/rancher/pkg/api/customization/pipeline"
	"github.com/rancher/rancher/pkg/api/customization/podsecuritypolicytemplate"
	projectStore "github.com/rancher/rancher/pkg/api/customization/project"
	projectaction "github.com/rancher/rancher/pkg/api/customization/project"
	"github.com/rancher/rancher/pkg/api/customization/roletemplate"
	"github.com/rancher/rancher/pkg/api/customization/roletemplatebinding"
	"github.com/rancher/rancher/pkg/api/customization/setting"
	appStore "github.com/rancher/rancher/pkg/api/store/app"
	"github.com/rancher/rancher/pkg/api/store/cert"
	"github.com/rancher/rancher/pkg/api/store/cluster"
	globaldnsAPIStore "github.com/rancher/rancher/pkg/api/store/globaldns"
	nodeStore "github.com/rancher/rancher/pkg/api/store/node"
	nodeTemplateStore "github.com/rancher/rancher/pkg/api/store/nodetemplate"
	"github.com/rancher/rancher/pkg/api/store/noopwatching"
	passwordStore "github.com/rancher/rancher/pkg/api/store/password"
	"github.com/rancher/rancher/pkg/api/store/preference"
	"github.com/rancher/rancher/pkg/api/store/scoped"
	settingstore "github.com/rancher/rancher/pkg/api/store/setting"
	"github.com/rancher/rancher/pkg/api/store/userscope"
	"github.com/rancher/rancher/pkg/auth/principals"
	"github.com/rancher/rancher/pkg/auth/providerrefresh"
	"github.com/rancher/rancher/pkg/auth/providers"
	"github.com/rancher/rancher/pkg/clustermanager"
	"github.com/rancher/rancher/pkg/controllers/management/compose/common"
	"github.com/rancher/rancher/pkg/nodeconfig"
	sourcecodeproviders "github.com/rancher/rancher/pkg/pipeline/providers"
	managementschema "github.com/rancher/types/apis/management.cattle.io/v3/schema"
	projectschema "github.com/rancher/types/apis/project.cattle.io/v3/schema"
	"github.com/rancher/types/client/management/v3"
	projectclient "github.com/rancher/types/client/project/v3"
	"github.com/rancher/types/config"
)

func Setup(ctx context.Context, apiContext *config.ScaledContext, clusterManager *clustermanager.Manager,
	k8sProxy http.Handler, localClusterEnabled bool) error {
	// Here we setup all types that will be stored in the Management cluster
	schemas := apiContext.Schemas

	factory := &crd.Factory{ClientGetter: apiContext.ClientGetter}

	factory.BatchCreateCRDs(ctx, config.ManagementStorageContext, schemas, &managementschema.Version,
		client.AuthConfigType,
		client.CatalogType,
		client.CatalogTemplateType,
		client.CatalogTemplateVersionType,
		client.ClusterAlertType,
		client.ClusterAlertGroupType,
		client.ClusterCatalogType,
		client.ClusterLoggingType,
		client.ClusterAlertRuleType,
		client.ClusterMonitorGraphType,
		client.ClusterRegistrationTokenType,
		client.ClusterRoleTemplateBindingType,
		client.ClusterType,
		client.ComposeConfigType,
		client.DynamicSchemaType,
		client.EtcdBackupType,
		client.GlobalRoleBindingType,
		client.GlobalRoleType,
		client.GroupMemberType,
		client.GroupType,
		client.KontainerDriverType,
		client.ListenConfigType,
		client.MultiClusterAppType,
		client.MultiClusterAppRevisionType,
		client.MonitorMetricType,
		client.NodeDriverType,
		client.NodePoolType,
		client.NodeTemplateType,
		client.NodeType,
		client.NotifierType,
		client.PodSecurityPolicyTemplateProjectBindingType,
		client.PodSecurityPolicyTemplateType,
		client.PreferenceType,
		client.ProjectAlertType,
		client.ProjectAlertGroupType,
		client.ProjectCatalogType,
		client.ProjectLoggingType,
		client.ProjectAlertRuleType,
		client.ProjectMonitorGraphType,
		client.ProjectNetworkPolicyType,
		client.ProjectRoleTemplateBindingType,
		client.ProjectType,
		client.RoleTemplateType,
		client.SettingType,
		client.TemplateType,
		client.TemplateVersionType,
		client.TemplateContentType,
		client.TokenType,
		client.UserAttributeType,
		client.UserType,
		client.GlobalDNSType,
		client.GlobalDNSProviderType)

	factory.BatchCreateCRDs(ctx, config.ManagementStorageContext, schemas, &projectschema.Version,
		projectclient.AppType,
		projectclient.AppRevisionType,
		projectclient.PipelineExecutionType,
		projectclient.PipelineSettingType,
		projectclient.PipelineType,
		projectclient.SourceCodeCredentialType,
		projectclient.SourceCodeProviderConfigType,
		projectclient.SourceCodeRepositoryType,
	)

	factory.BatchWait()

	Clusters(schemas, apiContext, clusterManager, k8sProxy)
	ClusterRoleTemplateBinding(schemas, apiContext)
	Templates(ctx, schemas, apiContext)
	TemplateVersion(ctx, schemas, apiContext)
	User(ctx, schemas, apiContext)
	Catalog(schemas, apiContext)
	ProjectCatalog(schemas, apiContext)
	ClusterCatalog(schemas, apiContext)
	SecretTypes(ctx, schemas, apiContext)
	App(schemas, apiContext, clusterManager)
	Setting(schemas)
	Preference(schemas, apiContext)
	ClusterRegistrationTokens(schemas)
	NodeTemplates(schemas, apiContext)
	LoggingTypes(schemas, apiContext, clusterManager, k8sProxy)
	Alert(schemas, apiContext)
	Pipeline(schemas, apiContext, clusterManager)
	Project(schemas, apiContext)
	ProjectRoleTemplateBinding(schemas, apiContext)
	TemplateContent(schemas)
	PodSecurityPolicyTemplate(schemas, apiContext)
	RoleTemplate(schemas, apiContext)
	MultiClusterApps(schemas, apiContext)
	GlobalDNSs(schemas, apiContext, localClusterEnabled)
	GlobalDNSProviders(schemas, apiContext, localClusterEnabled)
	Monitor(schemas, apiContext, clusterManager)
	KontainerDriver(schemas, apiContext)

	if err := NodeTypes(schemas, apiContext); err != nil {
		return err
	}

	principals.Schema(ctx, apiContext, schemas)
	providers.SetupAuthConfig(ctx, apiContext, schemas)
	authn.SetUserStore(schemas.Schema(&managementschema.Version, client.UserType), apiContext)
	authn.SetRTBStore(ctx, schemas.Schema(&managementschema.Version, client.ClusterRoleTemplateBindingType), apiContext)
	authn.SetRTBStore(ctx, schemas.Schema(&managementschema.Version, client.ProjectRoleTemplateBindingType), apiContext)
	nodeStore.SetupStore(schemas.Schema(&managementschema.Version, client.NodeType))
	projectStore.SetProjectStore(schemas.Schema(&managementschema.Version, client.ProjectType), apiContext)
	setupScopedTypes(schemas)
	setupPasswordTypes(ctx, schemas, apiContext)
	multiclusterapp.SetMemberStore(ctx, schemas.Schema(&managementschema.Version, client.MultiClusterAppType), apiContext)

	return nil
}

func setupPasswordTypes(ctx context.Context, schemas *types.Schemas, management *config.ScaledContext) {
	secretStore := management.Core.Secrets("")
	nsStore := management.Core.Namespaces("")
	passwordStore.SetPasswordStore(schemas, secretStore, nsStore)
}

func setupScopedTypes(schemas *types.Schemas) {
	for _, schema := range schemas.Schemas() {
		if schema.Scope != types.NamespaceScope || schema.Store == nil || schema.Store.Context() != config.ManagementStorageContext {
			continue
		}

		for _, key := range []string{"projectId", "clusterId"} {
			ns, ok := schema.ResourceFields["namespaceId"]
			if !ok {
				continue
			}

			if _, ok := schema.ResourceFields[key]; !ok {
				continue
			}

			schema.Store = scoped.NewScopedStore(key, schema.Store)
			ns.Required = false
			schema.ResourceFields["namespaceId"] = ns
			break
		}
	}
}

func Clusters(schemas *types.Schemas, managementContext *config.ScaledContext, clusterManager *clustermanager.Manager, k8sProxy http.Handler) {
	handler := ccluster.ActionHandler{
		NodepoolGetter:     managementContext.Management,
		ClusterClient:      managementContext.Management.Clusters(""),
		UserMgr:            managementContext.UserManager,
		ClusterManager:     clusterManager,
		NodeTemplateGetter: managementContext.Management,
		BackupClient:       managementContext.Management.EtcdBackups(""),
	}

	schema := schemas.Schema(&managementschema.Version, client.ClusterType)
	clusterFormatter := ccluster.Formatter{
		KontainerDriverLister: managementContext.Management.KontainerDrivers("").Controller().Lister(),
	}
	schema.Formatter = clusterFormatter.Formatter
	schema.ActionHandler = handler.ClusterActionHandler

	clusterValidator := ccluster.Validator{
		ClusterLister: managementContext.Management.Clusters("").Controller().Lister(),
	}
	schema.Validator = clusterValidator.Validator

	cluster.SetClusterStore(schema, managementContext, clusterManager, k8sProxy)
}

func Templates(ctx context.Context, schemas *types.Schemas, managementContext *config.ScaledContext) {
	schema := schemas.Schema(&managementschema.Version, client.TemplateType)
	schema.Scope = types.NamespaceScope
	schema.Store = proxy.NewProxyStore(ctx, managementContext.ClientGetter,
		config.ManagementStorageContext,
		[]string{"apis"},
		"management.cattle.io",
		"v3",
		"CatalogTemplate",
		"catalogtemplates")
	schema.Formatter = catalog.TemplateFormatter
	wrapper := catalog.TemplateWrapper{
		CatalogLister:        managementContext.Management.Catalogs("").Controller().Lister(),
		ClusterCatalogLister: managementContext.Management.ClusterCatalogs("").Controller().Lister(),
		ProjectCatalogLister: managementContext.Management.ProjectCatalogs("").Controller().Lister(),
	}
	schema.LinkHandler = wrapper.TemplateIconHandler

	schemaCatalogTemplate := schemas.Schema(&managementschema.Version, client.CatalogTemplateType)
	schemaCatalogTemplate.CollectionMethods = []string{}
	schemaCatalogTemplate.ResourceMethods = []string{}
}

func TemplateVersion(ctx context.Context, schemas *types.Schemas, managementContext *config.ScaledContext) {
	schema := schemas.Schema(&managementschema.Version, client.TemplateVersionType)
	schema.Scope = types.NamespaceScope
	schema.Store = proxy.NewProxyStore(ctx, managementContext.ClientGetter,
		config.ManagementStorageContext,
		[]string{"apis"},
		"management.cattle.io",
		"v3",
		"CatalogTemplateVersion",
		"catalogtemplateversions")
	t := catalog.TemplateVerionFormatterWrapper{
		CatalogLister:        managementContext.Management.Catalogs("").Controller().Lister(),
		ClusterCatalogLister: managementContext.Management.ClusterCatalogs("").Controller().Lister(),
		ProjectCatalogLister: managementContext.Management.ProjectCatalogs("").Controller().Lister(),
	}
	schema.Formatter = t.TemplateVersionFormatter
	schema.LinkHandler = t.TemplateVersionReadmeHandler
	schema.Store = noopwatching.Wrap(schema.Store)

	schemaCatalogTemplateVersion := schemas.Schema(&managementschema.Version, client.CatalogTemplateVersionType)
	schemaCatalogTemplateVersion.CollectionMethods = []string{}
	schemaCatalogTemplateVersion.ResourceMethods = []string{}
}

func TemplateContent(schemas *types.Schemas) {
	schema := schemas.Schema(&managementschema.Version, client.TemplateContentType)
	schema.Store = noopwatching.Wrap(schema.Store)
	schema.CollectionMethods = []string{}
}

func Catalog(schemas *types.Schemas, managementContext *config.ScaledContext) {
	schema := schemas.Schema(&managementschema.Version, client.CatalogType)
	schema.Formatter = catalog.Formatter
	handler := catalog.ActionHandler{
		CatalogClient: managementContext.Management.Catalogs(""),
	}
	schema.ActionHandler = handler.RefreshActionHandler
	schema.CollectionFormatter = catalog.CollectionFormatter
	schema.LinkHandler = handler.ExportYamlHandler
}

func ProjectCatalog(schemas *types.Schemas, managementContext *config.ScaledContext) {
	schema := schemas.Schema(&managementschema.Version, client.ProjectCatalogType)
	schema.Formatter = catalog.Formatter
	handler := catalog.ActionHandler{
		ProjectCatalogClient: managementContext.Management.ProjectCatalogs(""),
	}
	schema.ActionHandler = handler.RefreshProjectCatalogActionHandler
	schema.CollectionFormatter = catalog.CollectionFormatter
}

func ClusterCatalog(schemas *types.Schemas, managementContext *config.ScaledContext) {
	schema := schemas.Schema(&managementschema.Version, client.ClusterCatalogType)
	schema.Formatter = catalog.Formatter
	handler := catalog.ActionHandler{
		ClusterCatalogClient: managementContext.Management.ClusterCatalogs(""),
	}
	schema.ActionHandler = handler.RefreshClusterCatalogActionHandler
	schema.CollectionFormatter = catalog.CollectionFormatter
}

func ClusterRegistrationTokens(schemas *types.Schemas) {
	schema := schemas.Schema(&managementschema.Version, client.ClusterRegistrationTokenType)
	schema.Store = &cluster.RegistrationTokenStore{
		Store: schema.Store,
	}
	schema.Formatter = clusterregistrationtokens.Formatter
}

func NodeTemplates(schemas *types.Schemas, management *config.ScaledContext) {
	schema := schemas.Schema(&managementschema.Version, client.NodeTemplateType)
	npl := management.Management.NodePools("").Controller().Lister()
	f := nodetemplate.Formatter{
		NodePoolLister: npl,
	}
	schema.Formatter = f.Formatter
	s := &nodeTemplateStore.Store{
		Store:                 userscope.NewStore(management.Core.Namespaces(""), schema.Store),
		NodePoolLister:        npl,
		CloudCredentialLister: management.Core.Secrets(namespace.GlobalNamespace).Controller().Lister(),
	}
	schema.Store = s
	schema.Validator = nodetemplate.Validator
}

func SecretTypes(ctx context.Context, schemas *types.Schemas, management *config.ScaledContext) {
	secretSchema := schemas.Schema(&projectschema.Version, projectclient.SecretType)
	secretSchema.Store = proxy.NewProxyStore(ctx, management.ClientGetter,
		config.ManagementStorageContext,
		[]string{"api"},
		"",
		"v1",
		"Secret",
		"secrets")

	for _, subSchema := range schemas.SchemasForVersion(projectschema.Version) {
		if subSchema.BaseType == projectclient.SecretType && subSchema.ID != projectclient.SecretType {
			if subSchema.CanList(nil) == nil {
				subSchema.Store = subtype.NewSubTypeStore(subSchema.ID, secretSchema.Store)
			}
		}
	}

	secretSchema = schemas.Schema(&projectschema.Version, projectclient.CertificateType)
	secretSchema.Store = cert.Wrap(secretSchema.Store)

	mgmtSecretSchema := schemas.Schema(&managementschema.Version, client.ManagementSecretType)
	mgmtSecretSchema.Store = proxy.NewProxyStore(ctx, management.ClientGetter,
		config.ManagementStorageContext,
		[]string{"api"},
		"",
		"v1",
		"Secret",
		"secrets")

	credSchema := schemas.Schema(&managementschema.Version, client.CloudCredentialType)
	credSchema.Store = cred.Wrap(mgmtSecretSchema.Store,
		management.Core.Namespaces(""),
		management.Management.NodeTemplates("").Controller().Lister())
	credSchema.Validator = cred.Validator
}

func User(ctx context.Context, schemas *types.Schemas, management *config.ScaledContext) {
	schema := schemas.Schema(&managementschema.Version, client.UserType)
	handler := &authn.Handler{
		UserClient:               management.Management.Users(""),
		GlobalRoleBindingsClient: management.Management.GlobalRoleBindings(""),
		UserAuthRefresher:        providerrefresh.NewUserAuthRefresher(ctx, management),
	}

	schema.Formatter = handler.UserFormatter
	schema.CollectionFormatter = handler.CollectionFormatter
	schema.ActionHandler = handler.Actions
}

func Preference(schemas *types.Schemas, management *config.ScaledContext) {
	schema := schemas.Schema(&managementschema.Version, client.PreferenceType)
	schema.Store = preference.NewStore(management.Core.Namespaces(""), schema.Store)
}

func NodeTypes(schemas *types.Schemas, management *config.ScaledContext) error {
	secretStore, err := nodeconfig.NewStore(management.Core.Namespaces(""), management.Core)
	if err != nil {
		return err
	}

	schema := schemas.Schema(&managementschema.Version, client.NodeDriverType)
	machineDriverHandlers := &node.DriverHandlers{
		NodeDriverClient: management.Management.NodeDrivers(""),
	}
	schema.Formatter = machineDriverHandlers.Formatter
	schema.ActionHandler = machineDriverHandlers.ActionHandler
	schema.LinkHandler = machineDriverHandlers.ExportYamlHandler

	machineHandler := &node.Handler{
		SecretStore: secretStore,
	}

	schema = schemas.Schema(&managementschema.Version, client.NodeType)
	schema.Formatter = node.Formatter
	schema.LinkHandler = machineHandler.LinkHandler
	actionWrapper := node.ActionWrapper{}
	schema.ActionHandler = actionWrapper.ActionHandler
	return nil
}

func App(schemas *types.Schemas, management *config.ScaledContext, kubeConfigGetter common.KubeConfigGetter) {
	schema := schemas.Schema(&projectschema.Version, projectclient.AppType)
	store := &appStore.Store{
		Store: schema.Store,
		Apps:  management.Project.Apps("").Controller().Lister(),
	}
	schema.Store = store
	wrapper := app.Wrapper{
		Clusters:              management.Management.Clusters(""),
		TemplateVersionClient: management.Management.CatalogTemplateVersions(""),
		KubeConfigGetter:      kubeConfigGetter,
		AppGetter:             management.Project,
		UserLister:            management.Management.Users("").Controller().Lister(),
		UserManager:           management.UserManager,
	}
	schema.Formatter = app.Formatter
	schema.ActionHandler = wrapper.ActionHandler
	schema.LinkHandler = wrapper.LinkHandler
	schema.Validator = wrapper.Validator
}

func Setting(schemas *types.Schemas) {
	schema := schemas.Schema(&managementschema.Version, client.SettingType)
	schema.Formatter = setting.Formatter
	schema.Validator = setting.Validator
	schema.Store = settingstore.New(schema.Store)
}

func LoggingTypes(schemas *types.Schemas, management *config.ScaledContext, clusterManager *clustermanager.Manager, k8sProxy http.Handler) {
	handler := logging.NewHandler(
		management.Dialer,
		clusterManager,
		management.Project,
		management.Management.Projects("").Controller().Lister(),
		management.Management.ProjectLoggings("").Controller().Lister(),
		management.Management.CatalogTemplates("").Controller().Lister(),
	)

	schema := schemas.Schema(&managementschema.Version, client.ClusterLoggingType)
	schema.CollectionFormatter = logging.CollectionFormatter
	schema.ActionHandler = handler.ActionHandler
	schema.Validator = logging.ClusterLoggingValidator

	schema = schemas.Schema(&managementschema.Version, client.ProjectLoggingType)
	schema.CollectionFormatter = logging.CollectionFormatter
	schema.ActionHandler = handler.ActionHandler
	schema.Validator = logging.ProjectLoggingValidator
}

func Alert(schemas *types.Schemas, management *config.ScaledContext) {
	handler := &alert.Handler{
		ClusterAlertRule: management.Management.ClusterAlertRules(""),
		ProjectAlertRule: management.Management.ProjectAlertRules(""),
		Notifiers:        management.Management.Notifiers(""),
	}

	schema := schemas.Schema(&managementschema.Version, client.NotifierType)
	schema.CollectionFormatter = alert.NotifierCollectionFormatter
	schema.Formatter = alert.NotifierFormatter
	schema.ActionHandler = handler.NotifierActionHandler

	schema = schemas.Schema(&managementschema.Version, client.ClusterAlertRuleType)
	schema.Formatter = alert.RuleFormatter
	schema.Validator = alert.ClusterAlertRuleValidator
	schema.ActionHandler = handler.ClusterAlertRuleActionHandler

	schema = schemas.Schema(&managementschema.Version, client.ProjectAlertRuleType)
	schema.Formatter = alert.RuleFormatter
	schema.Validator = alert.ProjectAlertRuleValidator
	schema.ActionHandler = handler.ProjectAlertRuleActionHandler

	//old schema just for migrate
	schema = schemas.Schema(&managementschema.Version, client.ClusterAlertType)
	schema = schemas.Schema(&managementschema.Version, client.ProjectAlertType)
}

func Monitor(schemas *types.Schemas, management *config.ScaledContext, clusterManager *clustermanager.Manager) {
	clusterGraphHandler := monitor.NewClusterGraphHandler(management.Dialer, clusterManager)
	projectGraphHandler := monitor.NewProjectGraphHandler(management.Dialer, clusterManager)
	metricHandler := monitor.NewMetricHandler(management.Dialer, clusterManager)

	schema := schemas.Schema(&managementschema.Version, client.ClusterMonitorGraphType)
	schema.CollectionFormatter = monitor.QueryGraphCollectionFormatter
	schema.ActionHandler = clusterGraphHandler.QuerySeriesAction

	schema = schemas.Schema(&managementschema.Version, client.ProjectMonitorGraphType)
	schema.CollectionFormatter = monitor.QueryGraphCollectionFormatter
	schema.ActionHandler = projectGraphHandler.QuerySeriesAction

	schema = schemas.Schema(&managementschema.Version, client.MonitorMetricType)
	schema.CollectionFormatter = monitor.MetricCollectionFormatter
	schema.ActionHandler = metricHandler.Action
}

func Pipeline(schemas *types.Schemas, management *config.ScaledContext, clusterManager *clustermanager.Manager) {

	pipelineHandler := &pipeline.Handler{
		PipelineLister:             management.Project.Pipelines("").Controller().Lister(),
		PipelineExecutions:         management.Project.PipelineExecutions(""),
		SourceCodeCredentials:      management.Project.SourceCodeCredentials(""),
		SourceCodeCredentialLister: management.Project.SourceCodeCredentials("").Controller().Lister(),
	}
	schema := schemas.Schema(&projectschema.Version, projectclient.PipelineType)
	schema.Formatter = pipeline.Formatter
	schema.ActionHandler = pipelineHandler.ActionHandler
	schema.LinkHandler = pipelineHandler.LinkHandler

	pipelineExecutionHandler := &pipeline.ExecutionHandler{
		ClusterManager: clusterManager,

		PipelineLister:          management.Project.Pipelines("").Controller().Lister(),
		PipelineExecutionLister: management.Project.PipelineExecutions("").Controller().Lister(),
		PipelineExecutions:      management.Project.PipelineExecutions(""),
	}
	schema = schemas.Schema(&projectschema.Version, projectclient.PipelineExecutionType)
	schema.Formatter = pipelineExecutionHandler.ExecutionFormatter
	schema.LinkHandler = pipelineExecutionHandler.LinkHandler
	schema.ActionHandler = pipelineExecutionHandler.ActionHandler

	schema = schemas.Schema(&projectschema.Version, projectclient.PipelineSettingType)
	schema.Formatter = setting.PipelineFormatter

	sourceCodeCredentialHandler := &pipeline.SourceCodeCredentialHandler{
		SourceCodeCredentials:      management.Project.SourceCodeCredentials(""),
		SourceCodeCredentialLister: management.Project.SourceCodeCredentials("").Controller().Lister(),
		SourceCodeRepositories:     management.Project.SourceCodeRepositories(""),
		SourceCodeRepositoryLister: management.Project.SourceCodeRepositories("").Controller().Lister(),
	}
	schema = schemas.Schema(&projectschema.Version, projectclient.SourceCodeCredentialType)
	schema.Formatter = pipeline.SourceCodeCredentialFormatter
	schema.ListHandler = sourceCodeCredentialHandler.ListHandler
	schema.ActionHandler = sourceCodeCredentialHandler.ActionHandler
	schema.LinkHandler = sourceCodeCredentialHandler.LinkHandler
	schema.Store = userscope.NewStore(management.Core.Namespaces(""), schema.Store)

	schema = schemas.Schema(&projectschema.Version, projectclient.SourceCodeRepositoryType)
	schema.Store = userscope.NewStore(management.Core.Namespaces(""), schema.Store)

	//register and setup source code providers
	sourcecodeproviders.SetupSourceCodeProviderConfig(management, schemas)

}

func Project(schemas *types.Schemas, management *config.ScaledContext) {
	schema := schemas.Schema(&managementschema.Version, client.ProjectType)

	// delete monitoring resource action
	delete(schema.ResourceActions, "disableMonitoring")
	delete(schema.ResourceActions, "editMonitoring")
	delete(schema.ResourceActions, "enableMonitoring")
	delete(schema.ResourceActions, "viewMonitoring")

	schema.Formatter = projectaction.Formatter
	handler := &projectaction.Handler{
		Projects:       management.Management.Projects(""),
		ProjectLister:  management.Management.Projects("").Controller().Lister(),
		UserMgr:        management.UserManager,
		ClusterManager: management.ClientGetter.(*clustermanager.Manager),
	}
	schema.ActionHandler = handler.Actions
}

func PodSecurityPolicyTemplate(schemas *types.Schemas, management *config.ScaledContext) {
	schema := schemas.Schema(&managementschema.Version, client.PodSecurityPolicyTemplateType)
	schema.Formatter = podsecuritypolicytemplate.NewFormatter(management)
	schema.Store = &podsecuritypolicytemplate.Store{
		Store: schema.Store,
	}
	schema.Validator = podsecuritypolicytemplate.Validator
}

func ClusterRoleTemplateBinding(schemas *types.Schemas, management *config.ScaledContext) {
	schema := schemas.Schema(&managementschema.Version, client.ClusterRoleTemplateBindingType)
	schema.Validator = roletemplatebinding.NewCRTBValidator(management)
}

func ProjectRoleTemplateBinding(schemas *types.Schemas, management *config.ScaledContext) {
	schema := schemas.Schema(&managementschema.Version, client.ProjectRoleTemplateBindingType)
	schema.Validator = roletemplatebinding.NewPRTBValidator(management)
}

func RoleTemplate(schemas *types.Schemas, management *config.ScaledContext) {
	rt := roletemplate.Wrapper{
		RoleTemplateLister: management.Management.RoleTemplates("").Controller().Lister(),
	}
	schema := schemas.Schema(&managementschema.Version, client.RoleTemplateType)
	schema.Validator = rt.Validator
}

func KontainerDriver(schemas *types.Schemas, management *config.ScaledContext) {
	schema := schemas.Schema(&managementschema.Version, client.KontainerDriverType)
	handler := kontainerdriver.ActionHandler{
		KontainerDrivers:      management.Management.KontainerDrivers(""),
		KontainerDriverLister: management.Management.KontainerDrivers("").Controller().Lister(),
	}
	schema.ActionHandler = handler.ActionHandler
	schema.Formatter = kontainerdriver.NewFormatter(management)
	schema.Store = kontainerdriver.NewStore(management, schema.Store)
	kontainerDriverValidator := kontainerdriver.Validator{
		KontainerDriverLister: management.Management.KontainerDrivers("").Controller().Lister(),
	}
	schema.Validator = kontainerDriverValidator.Validator
}

func MultiClusterApps(schemas *types.Schemas, management *config.ScaledContext) {
	schema := schemas.Schema(&managementschema.Version, client.MultiClusterAppType)
	schema.Store = &globalresource.GlobalNamespaceStore{
		Store:              schema.Store,
		NamespaceInterface: management.Core.Namespaces(""),
	}
	revisionSchema := schemas.Schema(&managementschema.Version, client.MultiClusterAppRevisionType)
	revisionSchema.Store = &globalresource.GlobalNamespaceStore{
		Store:              revisionSchema.Store,
		NamespaceInterface: management.Core.Namespaces(""),
	}
	wrapper := multiclusterapp.Wrapper{
		MultiClusterApps:              management.Management.MultiClusterApps(""),
		MultiClusterAppLister:         management.Management.MultiClusterApps("").Controller().Lister(),
		MultiClusterAppRevisionLister: management.Management.MultiClusterAppRevisions("").Controller().Lister(),
		PrtbLister:                    management.Management.ProjectRoleTemplateBindings("").Controller().Lister(),
		CrtbLister:                    management.Management.ClusterRoleTemplateBindings("").Controller().Lister(),
		RoleTemplateLister:            management.Management.RoleTemplates("").Controller().Lister(),
		Users:                         management.Management.Users(""),
		GrbLister:                     management.Management.GlobalRoleBindings("").Controller().Lister(),
		GrLister:                      management.Management.GlobalRoles("").Controller().Lister(),
		Prtbs:                         management.Management.ProjectRoleTemplateBindings(""),
		Crtbs:                         management.Management.ClusterRoleTemplateBindings(""),
		ProjectLister:                 management.Management.Projects("").Controller().Lister(),
		ClusterLister:                 management.Management.Clusters("").Controller().Lister(),
	}
	schema.Formatter = wrapper.Formatter
	schema.ActionHandler = wrapper.ActionHandler
	schema.LinkHandler = wrapper.LinkHandler
	schema.Validator = wrapper.Validator
}

func GlobalDNSs(schemas *types.Schemas, management *config.ScaledContext, localClusterEnabled bool) {
	gdns := globaldns.Wrapper{
		GlobalDNSes:           management.Management.GlobalDNSs(""),
		GlobalDNSLister:       management.Management.GlobalDNSs("").Controller().Lister(),
		PrtbLister:            management.Management.ProjectRoleTemplateBindings("").Controller().Lister(),
		MultiClusterAppLister: management.Management.MultiClusterApps("").Controller().Lister(),
		Users:                 management.Management.Users(""),
		GrbLister:             management.Management.GlobalRoleBindings("").Controller().Lister(),
		GrLister:              management.Management.GlobalRoles("").Controller().Lister(),
	}
	schema := schemas.Schema(&managementschema.Version, client.GlobalDNSType)
	schema.Store = &globalresource.GlobalNamespaceStore{
		Store:              schema.Store,
		NamespaceInterface: management.Core.Namespaces(""),
	}
	schema.Formatter = gdns.Formatter
	schema.ActionHandler = gdns.ActionHandler
	schema.Validator = gdns.Validator
	schema.Store = globaldnsAPIStore.Wrap(schema.Store)
	if !localClusterEnabled {
		schema.CollectionMethods = []string{}
		schema.ResourceMethods = []string{}
	}
}

func GlobalDNSProviders(schemas *types.Schemas, management *config.ScaledContext, localClusterEnabled bool) {
	schema := schemas.Schema(&managementschema.Version, client.GlobalDNSProviderType)
	schema.Store = &globalresource.GlobalNamespaceStore{
		Store:              schema.Store,
		NamespaceInterface: management.Core.Namespaces(""),
	}
	schema.Store = globaldnsAPIStore.ProviderWrap(schema.Store)
	if !localClusterEnabled {
		schema.CollectionMethods = []string{}
		schema.ResourceMethods = []string{}
	}
}
