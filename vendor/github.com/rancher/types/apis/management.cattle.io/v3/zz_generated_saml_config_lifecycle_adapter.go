package v3

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/apimachinery/pkg/runtime"
)

type SamlConfigLifecycle interface {
	Create(obj *SamlConfig) (*SamlConfig, error)
	Remove(obj *SamlConfig) (*SamlConfig, error)
	Updated(obj *SamlConfig) (*SamlConfig, error)
}

type samlConfigLifecycleAdapter struct {
	lifecycle SamlConfigLifecycle
}

func (w *samlConfigLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*SamlConfig))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *samlConfigLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*SamlConfig))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *samlConfigLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*SamlConfig))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewSamlConfigLifecycleAdapter(name string, clusterScoped bool, client SamlConfigInterface, l SamlConfigLifecycle) SamlConfigHandlerFunc {
	adapter := &samlConfigLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *SamlConfig) error {
		if obj == nil {
			return syncFn(key, nil)
		}
		return syncFn(key, obj)
	}
}
