package client

import (
	"github.com/rancher/norman/types"
)

const (
	SamlConfigType                          = "samlConfig"
	SamlConfigFieldAccessMode               = "accessMode"
	SamlConfigFieldAllowedPrincipalIDs      = "allowedPrincipalIds"
	SamlConfigFieldAnnotations              = "annotations"
	SamlConfigFieldCreated                  = "created"
	SamlConfigFieldCreatorID                = "creatorId"
	SamlConfigFieldDisplayNameField         = "displayNameField"
	SamlConfigFieldEnabled                  = "enabled"
	SamlConfigFieldGroupsField              = "groupsField"
	SamlConfigFieldIDPMetadataContent       = "idpMetadataContent"
	SamlConfigFieldIDPMetadataFilePath      = "idpMetadataFilePath"
	SamlConfigFieldIDPMetadataURL           = "idpMetadataUrl"
	SamlConfigFieldLabels                   = "labels"
	SamlConfigFieldName                     = "name"
	SamlConfigFieldOwnerReferences          = "ownerReferences"
	SamlConfigFieldRancherAPIHost           = "rancherApiHost"
	SamlConfigFieldRemoved                  = "removed"
	SamlConfigFieldSPSelfSignedCert         = "spCert"
	SamlConfigFieldSPSelfSignedCertFilePath = "spSelfSignedCertFilePath"
	SamlConfigFieldSPSelfSignedKey          = "spKey"
	SamlConfigFieldSPSelfSignedKeyFilePath  = "spSelfSignedKeyFilePath"
	SamlConfigFieldType                     = "type"
	SamlConfigFieldUIDField                 = "uidField"
	SamlConfigFieldUserNameField            = "userNameField"
	SamlConfigFieldUuid                     = "uuid"
)

type SamlConfig struct {
	types.Resource
	AccessMode               string            `json:"accessMode,omitempty" yaml:"accessMode,omitempty"`
	AllowedPrincipalIDs      []string          `json:"allowedPrincipalIds,omitempty" yaml:"allowedPrincipalIds,omitempty"`
	Annotations              map[string]string `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	Created                  string            `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID                string            `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	DisplayNameField         string            `json:"displayNameField,omitempty" yaml:"displayNameField,omitempty"`
	Enabled                  bool              `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	GroupsField              string            `json:"groupsField,omitempty" yaml:"groupsField,omitempty"`
	IDPMetadataContent       string            `json:"idpMetadataContent,omitempty" yaml:"idpMetadataContent,omitempty"`
	IDPMetadataFilePath      string            `json:"idpMetadataFilePath,omitempty" yaml:"idpMetadataFilePath,omitempty"`
	IDPMetadataURL           string            `json:"idpMetadataUrl,omitempty" yaml:"idpMetadataUrl,omitempty"`
	Labels                   map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	Name                     string            `json:"name,omitempty" yaml:"name,omitempty"`
	OwnerReferences          []OwnerReference  `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	RancherAPIHost           string            `json:"rancherApiHost,omitempty" yaml:"rancherApiHost,omitempty"`
	Removed                  string            `json:"removed,omitempty" yaml:"removed,omitempty"`
	SPSelfSignedCert         string            `json:"spCert,omitempty" yaml:"spCert,omitempty"`
	SPSelfSignedCertFilePath string            `json:"spSelfSignedCertFilePath,omitempty" yaml:"spSelfSignedCertFilePath,omitempty"`
	SPSelfSignedKey          string            `json:"spKey,omitempty" yaml:"spKey,omitempty"`
	SPSelfSignedKeyFilePath  string            `json:"spSelfSignedKeyFilePath,omitempty" yaml:"spSelfSignedKeyFilePath,omitempty"`
	Type                     string            `json:"type,omitempty" yaml:"type,omitempty"`
	UIDField                 string            `json:"uidField,omitempty" yaml:"uidField,omitempty"`
	UserNameField            string            `json:"userNameField,omitempty" yaml:"userNameField,omitempty"`
	Uuid                     string            `json:"uuid,omitempty" yaml:"uuid,omitempty"`
}
type SamlConfigCollection struct {
	types.Collection
	Data   []SamlConfig `json:"data,omitempty"`
	client *SamlConfigClient
}

type SamlConfigClient struct {
	apiClient *Client
}

type SamlConfigOperations interface {
	List(opts *types.ListOpts) (*SamlConfigCollection, error)
	Create(opts *SamlConfig) (*SamlConfig, error)
	Update(existing *SamlConfig, updates interface{}) (*SamlConfig, error)
	ByID(id string) (*SamlConfig, error)
	Delete(container *SamlConfig) error
}

func newSamlConfigClient(apiClient *Client) *SamlConfigClient {
	return &SamlConfigClient{
		apiClient: apiClient,
	}
}

func (c *SamlConfigClient) Create(container *SamlConfig) (*SamlConfig, error) {
	resp := &SamlConfig{}
	err := c.apiClient.Ops.DoCreate(SamlConfigType, container, resp)
	return resp, err
}

func (c *SamlConfigClient) Update(existing *SamlConfig, updates interface{}) (*SamlConfig, error) {
	resp := &SamlConfig{}
	err := c.apiClient.Ops.DoUpdate(SamlConfigType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *SamlConfigClient) List(opts *types.ListOpts) (*SamlConfigCollection, error) {
	resp := &SamlConfigCollection{}
	err := c.apiClient.Ops.DoList(SamlConfigType, opts, resp)
	resp.client = c
	return resp, err
}

func (cc *SamlConfigCollection) Next() (*SamlConfigCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &SamlConfigCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *SamlConfigClient) ByID(id string) (*SamlConfig, error) {
	resp := &SamlConfig{}
	err := c.apiClient.Ops.DoByID(SamlConfigType, id, resp)
	return resp, err
}

func (c *SamlConfigClient) Delete(container *SamlConfig) error {
	return c.apiClient.Ops.DoResourceDelete(SamlConfigType, &container.Resource)
}
