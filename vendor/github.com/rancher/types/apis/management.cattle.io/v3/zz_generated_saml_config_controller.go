package v3

import (
	"context"

	"github.com/rancher/norman/controller"
	"github.com/rancher/norman/objectclient"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
)

var (
	SamlConfigGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "SamlConfig",
	}
	SamlConfigResource = metav1.APIResource{
		Name:         "samlconfigs",
		SingularName: "samlconfig",
		Namespaced:   false,
		Kind:         SamlConfigGroupVersionKind.Kind,
	}
)

type SamlConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SamlConfig
}

type SamlConfigHandlerFunc func(key string, obj *SamlConfig) error

type SamlConfigLister interface {
	List(namespace string, selector labels.Selector) (ret []*SamlConfig, err error)
	Get(namespace, name string) (*SamlConfig, error)
}

type SamlConfigController interface {
	Informer() cache.SharedIndexInformer
	Lister() SamlConfigLister
	AddHandler(name string, handler SamlConfigHandlerFunc)
	AddClusterScopedHandler(name, clusterName string, handler SamlConfigHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type SamlConfigInterface interface {
	ObjectClient() *objectclient.ObjectClient
	Create(*SamlConfig) (*SamlConfig, error)
	GetNamespaced(namespace, name string, opts metav1.GetOptions) (*SamlConfig, error)
	Get(name string, opts metav1.GetOptions) (*SamlConfig, error)
	Update(*SamlConfig) (*SamlConfig, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*SamlConfigList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() SamlConfigController
	AddHandler(name string, sync SamlConfigHandlerFunc)
	AddLifecycle(name string, lifecycle SamlConfigLifecycle)
	AddClusterScopedHandler(name, clusterName string, sync SamlConfigHandlerFunc)
	AddClusterScopedLifecycle(name, clusterName string, lifecycle SamlConfigLifecycle)
}

type samlConfigLister struct {
	controller *samlConfigController
}

func (l *samlConfigLister) List(namespace string, selector labels.Selector) (ret []*SamlConfig, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*SamlConfig))
	})
	return
}

func (l *samlConfigLister) Get(namespace, name string) (*SamlConfig, error) {
	var key string
	if namespace != "" {
		key = namespace + "/" + name
	} else {
		key = name
	}
	obj, exists, err := l.controller.Informer().GetIndexer().GetByKey(key)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(schema.GroupResource{
			Group:    SamlConfigGroupVersionKind.Group,
			Resource: "samlConfig",
		}, name)
	}
	return obj.(*SamlConfig), nil
}

type samlConfigController struct {
	controller.GenericController
}

func (c *samlConfigController) Lister() SamlConfigLister {
	return &samlConfigLister{
		controller: c,
	}
}

func (c *samlConfigController) AddHandler(name string, handler SamlConfigHandlerFunc) {
	c.GenericController.AddHandler(name, func(key string) error {
		obj, exists, err := c.Informer().GetStore().GetByKey(key)
		if err != nil {
			return err
		}
		if !exists {
			return handler(key, nil)
		}
		return handler(key, obj.(*SamlConfig))
	})
}

func (c *samlConfigController) AddClusterScopedHandler(name, cluster string, handler SamlConfigHandlerFunc) {
	c.GenericController.AddHandler(name, func(key string) error {
		obj, exists, err := c.Informer().GetStore().GetByKey(key)
		if err != nil {
			return err
		}
		if !exists {
			return handler(key, nil)
		}

		if !controller.ObjectInCluster(cluster, obj) {
			return nil
		}

		return handler(key, obj.(*SamlConfig))
	})
}

type samlConfigFactory struct {
}

func (c samlConfigFactory) Object() runtime.Object {
	return &SamlConfig{}
}

func (c samlConfigFactory) List() runtime.Object {
	return &SamlConfigList{}
}

func (s *samlConfigClient) Controller() SamlConfigController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.samlConfigControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(SamlConfigGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &samlConfigController{
		GenericController: genericController,
	}

	s.client.samlConfigControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type samlConfigClient struct {
	client       *Client
	ns           string
	objectClient *objectclient.ObjectClient
	controller   SamlConfigController
}

func (s *samlConfigClient) ObjectClient() *objectclient.ObjectClient {
	return s.objectClient
}

func (s *samlConfigClient) Create(o *SamlConfig) (*SamlConfig, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*SamlConfig), err
}

func (s *samlConfigClient) Get(name string, opts metav1.GetOptions) (*SamlConfig, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*SamlConfig), err
}

func (s *samlConfigClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (*SamlConfig, error) {
	obj, err := s.objectClient.GetNamespaced(namespace, name, opts)
	return obj.(*SamlConfig), err
}

func (s *samlConfigClient) Update(o *SamlConfig) (*SamlConfig, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*SamlConfig), err
}

func (s *samlConfigClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *samlConfigClient) DeleteNamespaced(namespace, name string, options *metav1.DeleteOptions) error {
	return s.objectClient.DeleteNamespaced(namespace, name, options)
}

func (s *samlConfigClient) List(opts metav1.ListOptions) (*SamlConfigList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*SamlConfigList), err
}

func (s *samlConfigClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *samlConfigClient) Patch(o *SamlConfig, data []byte, subresources ...string) (*SamlConfig, error) {
	obj, err := s.objectClient.Patch(o.Name, o, data, subresources...)
	return obj.(*SamlConfig), err
}

func (s *samlConfigClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *samlConfigClient) AddHandler(name string, sync SamlConfigHandlerFunc) {
	s.Controller().AddHandler(name, sync)
}

func (s *samlConfigClient) AddLifecycle(name string, lifecycle SamlConfigLifecycle) {
	sync := NewSamlConfigLifecycleAdapter(name, false, s, lifecycle)
	s.AddHandler(name, sync)
}

func (s *samlConfigClient) AddClusterScopedHandler(name, clusterName string, sync SamlConfigHandlerFunc) {
	s.Controller().AddClusterScopedHandler(name, clusterName, sync)
}

func (s *samlConfigClient) AddClusterScopedLifecycle(name, clusterName string, lifecycle SamlConfigLifecycle) {
	sync := NewSamlConfigLifecycleAdapter(name+"_"+clusterName, true, s, lifecycle)
	s.AddClusterScopedHandler(name, clusterName, sync)
}
