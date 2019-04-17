package auth

import (
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	"github.com/pkg/errors"
	"github.com/rancher/norman/condition"
	systemimage "github.com/rancher/rancher/pkg/controllers/user/systemimage"
	"github.com/rancher/rancher/pkg/project"
	corev1 "github.com/rancher/types/apis/core/v1"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	rrbacv1 "github.com/rancher/types/apis/rbac.authorization.k8s.io/v1"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	v12 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	creatorIDAnn                  = "field.cattle.io/creatorId"
	creatorOwnerBindingAnnotation = "authz.management.cattle.io/creator-owner-binding"
	projectCreateController       = "mgmt-project-rbac-create"
	clusterCreateController       = "mgmt-cluster-rbac-delete" // TODO the word delete here is wrong, but changing it would break backwards compatibility
	projectRemoveController       = "mgmt-project-rbac-remove"
	clusterRemoveController       = "mgmt-cluster-rbac-remove"
	roleTemplatesRequired         = "authz.management.cattle.io/creator-role-bindings"
)

var defaultProjectLabels = labels.Set(map[string]string{"authz.management.cattle.io/default-project": "true"})
var systemProjectLabels = labels.Set(map[string]string{"authz.management.cattle.io/system-project": "true"})
var crtbCeatorOwnerAnnotations = map[string]string{creatorOwnerBindingAnnotation: "true"}

var defaultProjects = map[string]bool{
	project.Default: true,
}
var systemProjects = map[string]bool{
	project.System: true,
}

func newPandCLifecycles(management *config.ManagementContext) (*projectLifecycle, *clusterLifecycle) {
	m := &mgr{
		mgmt:               management,
		nsLister:           management.Core.Namespaces("").Controller().Lister(),
		prtbLister:         management.Management.ProjectRoleTemplateBindings("").Controller().Lister(),
		crtbLister:         management.Management.ClusterRoleTemplateBindings("").Controller().Lister(),
		projectLister:      management.Management.Projects("").Controller().Lister(),
		roleTemplateLister: management.Management.RoleTemplates("").Controller().Lister(),
		clusterRoleClient:  management.RBAC.ClusterRoles(""),
	}
	p := &projectLifecycle{
		mgr: m,
	}
	c := &clusterLifecycle{
		mgr: m,
	}
	return p, c
}

type projectLifecycle struct {
	mgr *mgr
}

func (l *projectLifecycle) sync(key string, orig *v3.Project) (runtime.Object, error) {
	if orig == nil {
		return nil, nil
	}

	obj := orig.DeepCopyObject()

	obj, err := l.mgr.reconcileResourceToNamespace(obj, projectCreateController)
	if err != nil {
		return nil, err
	}

	obj, err = l.mgr.reconcileCreatorRTB(obj)
	if err != nil {
		return nil, err
	}

	// update if it has changed
	if obj != nil && !reflect.DeepEqual(orig, obj) {
		logrus.Infof("[%v] Updating project %v", projectCreateController, orig.Name)
		_, err = l.mgr.mgmt.Management.Projects("").ObjectClient().Update(orig.Name, obj)
		if err != nil {
			return nil, err
		}
	}
	if err != nil && !kerrors.IsAlreadyExists(err) {
		return nil, err
	}

	return nil, nil
}

func (l *projectLifecycle) Create(obj *v3.Project) (runtime.Object, error) {
	// no-op because the sync function will take care of it
	return obj, nil
}

func (l *projectLifecycle) Updated(obj *v3.Project) (runtime.Object, error) {
	// no-op because the sync function will take care of it
	return obj, nil
}

func (l *projectLifecycle) Remove(obj *v3.Project) (runtime.Object, error) {
	err := l.mgr.deleteNamespace(obj, projectRemoveController)
	return obj, err
}

type clusterLifecycle struct {
	mgr *mgr
}

func (l *clusterLifecycle) sync(key string, orig *v3.Cluster) (runtime.Object, error) {
	if orig == nil {
		return nil, nil
	}

	obj := orig.DeepCopyObject()
	obj, err := l.mgr.reconcileResourceToNamespace(obj, clusterCreateController)
	if err != nil {
		return nil, err
	}

	obj, err = l.mgr.createDefaultProject(obj)
	if err != nil {
		return nil, err
	}

	obj, err = l.mgr.createSystemProject(obj)
	if err != nil {
		return nil, err
	}
	obj, err = l.mgr.addRTAnnotation(obj, "cluster")
	if err != nil {
		return nil, err
	}

	// update if it has changed
	if obj != nil && !reflect.DeepEqual(orig, obj) {
		logrus.Infof("[%v] Updating cluster %v", clusterCreateController, orig.Name)
		_, err = l.mgr.mgmt.Management.Clusters("").ObjectClient().Update(orig.Name, obj)
		if err != nil {
			return nil, err
		}
	}

	obj, err = l.mgr.reconcileCreatorRTB(obj)
	if err != nil {
		return nil, err
	}

	// update if it has changed
	if obj != nil && !reflect.DeepEqual(orig, obj) {
		logrus.Infof("[%v] Updating cluster %v", clusterCreateController, orig.Name)
		_, err = l.mgr.mgmt.Management.Clusters("").ObjectClient().Update(orig.Name, obj)
		if err != nil {
			return nil, err
		}
	}

	return nil, nil
}

func (l *clusterLifecycle) Create(obj *v3.Cluster) (runtime.Object, error) {
	// no-op because the sync function will take care of it
	return obj, nil
}

func (l *clusterLifecycle) Updated(obj *v3.Cluster) (runtime.Object, error) {
	// no-op because the sync function will take care of it
	return obj, nil
}

func (l *clusterLifecycle) Remove(obj *v3.Cluster) (runtime.Object, error) {
	err := l.mgr.deleteNamespace(obj, clusterRemoveController)
	return obj, err
}

type mgr struct {
	mgmt          *config.ManagementContext
	nsLister      corev1.NamespaceLister
	projectLister v3.ProjectLister

	prtbLister         v3.ProjectRoleTemplateBindingLister
	crtbLister         v3.ClusterRoleTemplateBindingLister
	roleTemplateLister v3.RoleTemplateLister
	clusterRoleClient  rrbacv1.ClusterRoleInterface
}

func (m *mgr) createDefaultProject(obj runtime.Object) (runtime.Object, error) {
	return m.createProject(project.Default, v3.ClusterConditionconditionDefaultProjectCreated, obj, defaultProjectLabels, defaultProjects)
}

func (m *mgr) createSystemProject(obj runtime.Object) (runtime.Object, error) {
	return m.createProject(project.System, v3.ClusterConditionconditionSystemProjectCreated, obj, systemProjectLabels, systemProjects)
}

func (m *mgr) createProject(name string, cond condition.Cond, obj runtime.Object, labels labels.Set, projectMap map[string]bool) (runtime.Object, error) {
	return cond.DoUntilTrue(obj, func() (runtime.Object, error) {
		metaAccessor, err := meta.Accessor(obj)
		if err != nil {
			return obj, err
		}
		projects, err := m.projectLister.List(metaAccessor.GetName(), labels.AsSelector())
		if err != nil {
			return obj, err
		}
		if len(projects) > 0 {
			return obj, nil
		}
		creatorID, ok := metaAccessor.GetAnnotations()[creatorIDAnn]
		if !ok {
			logrus.Warnf("Cluster %v has no creatorId annotation. Cannot create %s project", metaAccessor.GetName(), name)
			return obj, nil
		}

		annotation := map[string]string{
			creatorIDAnn: creatorID,
		}

		if name == project.System {
			latestSystemVersion, err := systemimage.GetSystemImageVersion()
			if err != nil {
				return obj, err
			}
			annotation[project.SystemImageVersionAnn] = latestSystemVersion
		}

		project := &v3.Project{
			ObjectMeta: v1.ObjectMeta{
				GenerateName: "p-",
				Annotations:  annotation,
				Labels:       labels,
			},
			Spec: v3.ProjectSpec{
				DisplayName: name,
				Description: fmt.Sprintf("%s project created for the cluster", name),
				ClusterName: metaAccessor.GetName(),
			},
		}
		updated, err := m.addRTAnnotation(project, "project")
		if err != nil {
			return obj, err
		}
		project = updated.(*v3.Project)
		logrus.Infof("[%v] Creating %s project for cluster %v", clusterCreateController, name, metaAccessor.GetName())
		if _, err = m.mgmt.Management.Projects(metaAccessor.GetName()).Create(project); err != nil {
			return obj, err
		}
		return obj, nil
	})
}

func (m *mgr) reconcileCreatorRTB(obj runtime.Object) (runtime.Object, error) {
	return v3.CreatorMadeOwner.DoUntilTrue(obj, func() (runtime.Object, error) {
		metaAccessor, err := meta.Accessor(obj)
		if err != nil {
			return obj, err
		}

		typeAccessor, err := meta.TypeAccessor(obj)
		if err != nil {
			return obj, err
		}

		creatorID, ok := metaAccessor.GetAnnotations()[creatorIDAnn]
		if !ok || creatorID == "" {
			logrus.Warnf("%v %v has no creatorId annotation. Cannot add creator as owner", typeAccessor.GetKind(), metaAccessor.GetName())
			return obj, nil
		}

		switch typeAccessor.GetKind() {
		case v3.ProjectGroupVersionKind.Kind:
			project := obj.(*v3.Project)

			if v3.ProjectConditionInitialRolesPopulated.IsTrue(project) {
				// The projectRoleBindings are already completed, no need to check
				break
			}

			// If the project does not have the annotation it indicates the
			// project is from a previous rancher version so don't add the
			// default bindings.
			roleJSON, ok := project.Annotations[roleTemplatesRequired]
			if !ok {
				return project, nil
			}

			roleMap := make(map[string][]string)
			err = json.Unmarshal([]byte(roleJSON), &roleMap)
			if err != nil {
				return obj, err
			}

			var createdRoles []string

			for _, role := range roleMap["required"] {
				rtbName := "creator-" + role

				if rtb, _ := m.prtbLister.Get(metaAccessor.GetName(), rtbName); rtb != nil {
					createdRoles = append(createdRoles, role)
					// This projectRoleBinding exists, need to check all of them so keep going
					continue
				}

				// The projectRoleBinding doesn't exist yet so create it
				om := v1.ObjectMeta{
					Name:      rtbName,
					Namespace: metaAccessor.GetName(),
				}

				logrus.Infof("[%v] Creating creator projectRoleTemplateBinding for user %v for project %v", projectCreateController, creatorID, metaAccessor.GetName())
				if _, err := m.mgmt.Management.ProjectRoleTemplateBindings(metaAccessor.GetName()).Create(&v3.ProjectRoleTemplateBinding{
					ObjectMeta:       om,
					ProjectName:      metaAccessor.GetNamespace() + ":" + metaAccessor.GetName(),
					RoleTemplateName: role,
					UserName:         creatorID,
				}); err != nil && !apierrors.IsAlreadyExists(err) {
					return obj, err
				}
				createdRoles = append(createdRoles, role)
			}

			project = project.DeepCopy()

			roleMap["created"] = createdRoles
			d, err := json.Marshal(roleMap)
			if err != nil {
				return obj, err
			}

			project.Annotations[roleTemplatesRequired] = string(d)

			if reflect.DeepEqual(roleMap["required"], createdRoles) {
				v3.ProjectConditionInitialRolesPopulated.True(project)
				logrus.Infof("[%v] Setting InitialRolesPopulated condition on project %v", ctrbMGMTController, project.Name)
			}
			if _, err := m.mgmt.Management.Projects("").Update(project); err != nil {
				return obj, err
			}

		case v3.ClusterGroupVersionKind.Kind:
			cluster := obj.(*v3.Cluster)

			if v3.ClusterConditionInitialRolesPopulated.IsTrue(cluster) {
				// The clusterRoleBindings are already completed, no need to check
				break
			}

			roleJSON, ok := cluster.Annotations[roleTemplatesRequired]
			if !ok {
				return cluster, nil
			}

			roleMap := make(map[string][]string)
			err = json.Unmarshal([]byte(roleJSON), &roleMap)
			if err != nil {
				return obj, err
			}

			var createdRoles []string

			for _, role := range roleMap["required"] {
				rtbName := "creator-" + role

				if rtb, _ := m.crtbLister.Get(metaAccessor.GetName(), rtbName); rtb != nil {
					createdRoles = append(createdRoles, role)
					// This clusterRoleBinding exists, need to check all of them so keep going
					continue
				}

				// The clusterRoleBinding doesn't exist yet so create it
				om := v1.ObjectMeta{
					Name:      rtbName,
					Namespace: metaAccessor.GetName(),
				}
				om.Annotations = crtbCeatorOwnerAnnotations

				logrus.Infof("[%v] Creating creator clusterRoleTemplateBinding for user %v for cluster %v", projectCreateController, creatorID, metaAccessor.GetName())
				if _, err := m.mgmt.Management.ClusterRoleTemplateBindings(metaAccessor.GetName()).Create(&v3.ClusterRoleTemplateBinding{
					ObjectMeta:       om,
					ClusterName:      metaAccessor.GetName(),
					RoleTemplateName: role,
					UserName:         creatorID,
				}); err != nil && !apierrors.IsAlreadyExists(err) {
					return obj, err
				}
				createdRoles = append(createdRoles, role)
			}

			roleMap["created"] = createdRoles
			d, err := json.Marshal(roleMap)
			if err != nil {
				return obj, err
			}

			updateCondition := reflect.DeepEqual(roleMap["required"], createdRoles)

			err = m.updateClusterAnnotationandCondition(cluster, string(d), updateCondition)
			if err != nil {
				return obj, err
			}
		}

		return obj, nil
	})
}

func (m *mgr) deleteNamespace(obj runtime.Object, controller string) error {
	o, err := meta.Accessor(obj)
	if err != nil {
		return condition.Error("MissingMetadata", err)
	}

	nsClient := m.mgmt.K8sClient.CoreV1().Namespaces()
	ns, err := nsClient.Get(o.GetName(), v1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return nil
	}
	if ns.Status.Phase != v12.NamespaceTerminating {
		logrus.Infof("[%v] Deleting namespace %v", controller, o.GetName())
		err = nsClient.Delete(o.GetName(), nil)
		if apierrors.IsNotFound(err) {
			return nil
		}
	}
	return err
}

func (m *mgr) reconcileResourceToNamespace(obj runtime.Object, controller string) (runtime.Object, error) {
	return v3.NamespaceBackedResource.Do(obj, func() (runtime.Object, error) {
		o, err := meta.Accessor(obj)
		if err != nil {
			return obj, condition.Error("MissingMetadata", err)
		}
		t, err := meta.TypeAccessor(obj)
		if err != nil {
			return obj, condition.Error("MissingTypeMetadata", err)
		}

		ns, _ := m.nsLister.Get("", o.GetName())
		if ns == nil {
			nsClient := m.mgmt.K8sClient.CoreV1().Namespaces()
			logrus.Infof("[%v] Creating namespace %v", controller, o.GetName())
			_, err := nsClient.Create(&v12.Namespace{
				ObjectMeta: v1.ObjectMeta{
					Name: o.GetName(),
					Annotations: map[string]string{
						"management.cattle.io/system-namespace": "true",
					},
				},
			})
			if err != nil {
				return obj, condition.Error("NamespaceCreationFailure", errors.Wrapf(err, "failed to create namespace for %v %v", t.GetKind(), o.GetName()))
			}
		}

		return obj, nil
	})
}

func (m *mgr) addRTAnnotation(obj runtime.Object, context string) (runtime.Object, error) {
	meta, err := meta.Accessor(obj)
	if err != nil {
		return obj, err
	}

	// If the annotation is already there move along
	if _, ok := meta.GetAnnotations()[roleTemplatesRequired]; ok {
		return obj, nil
	}

	rt, err := m.roleTemplateLister.List("", labels.NewSelector())
	if err != nil {
		return obj, err
	}

	annoMap := make(map[string][]string)

	switch context {
	case "project":
		for _, role := range rt {
			if role.ProjectCreatorDefault && !role.Locked {
				annoMap["required"] = append(annoMap["required"], role.Name)
			}
		}
	case "cluster":
		for _, role := range rt {
			if role.ClusterCreatorDefault && !role.Locked {
				annoMap["required"] = append(annoMap["required"], role.Name)
			}
		}
		annoMap["created"] = []string{}
	}

	d, err := json.Marshal(annoMap)
	if err != nil {
		return obj, err
	}

	// Save the required role templates to the annotation on the obj
	meta.GetAnnotations()[roleTemplatesRequired] = string(d)

	return obj, nil
}

func (m *mgr) updateClusterAnnotationandCondition(cluster *v3.Cluster, anno string, updateCondition bool) error {
	sleep := 100
	for i := 0; i <= 3; i++ {
		c, err := m.mgmt.Management.Clusters("").Get(cluster.Name, v1.GetOptions{})
		if err != nil {
			return err
		}

		c.Annotations[roleTemplatesRequired] = anno

		if updateCondition {
			v3.ClusterConditionInitialRolesPopulated.True(c)
		}
		_, err = m.mgmt.Management.Clusters("").Update(c)
		if err != nil {
			if apierrors.IsConflict(err) {
				time.Sleep(time.Duration(sleep) * time.Millisecond)
				sleep *= 2
				continue
			}
			return err
		}
		// Only log if we successfully updated the cluster
		if updateCondition {
			logrus.Infof("[%v] Setting InitialRolesPopulated condition on cluster %v", ctrbMGMTController, c.ClusterName)
		}
		return nil
	}
	return nil
}
