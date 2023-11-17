/*
 * CLOUD API
 *
 * IONOS Enterprise-grade Infrastructure as a Service (IaaS) solutions can be managed through the Cloud API, in addition or as an alternative to the \"Data Center Designer\" (DCD) browser-based tool.    Both methods employ consistent concepts and features, deliver similar power and flexibility, and can be used to perform a multitude of management tasks, including adding servers, volumes, configuring networks, and so on.
 *
 * API version: 6.0
 */

// Code generated by OpenAPI Generator (https://openapi-generator.tech); DO NOT EDIT.

package ionoscloud

import (
	"encoding/json"
)

// KubernetesCluster struct for KubernetesCluster
type KubernetesCluster struct {
	Entities *KubernetesClusterEntities `json:"entities,omitempty"`
	// The URL to the object representation (absolute path).
	Href *string `json:"href,omitempty"`
	// The resource unique identifier.
	Id         *string                      `json:"id,omitempty"`
	Metadata   *DatacenterElementMetadata   `json:"metadata,omitempty"`
	Properties *KubernetesClusterProperties `json:"properties"`
	// The object type.
	Type *string `json:"type,omitempty"`
}

// NewKubernetesCluster instantiates a new KubernetesCluster object
// This constructor will assign default values to properties that have it defined,
// and makes sure properties required by API are set, but the set of arguments
// will change when the set of required properties is changed
func NewKubernetesCluster(properties KubernetesClusterProperties) *KubernetesCluster {
	this := KubernetesCluster{}

	this.Properties = &properties

	return &this
}

// NewKubernetesClusterWithDefaults instantiates a new KubernetesCluster object
// This constructor will only assign default values to properties that have it defined,
// but it doesn't guarantee that properties required by API are set
func NewKubernetesClusterWithDefaults() *KubernetesCluster {
	this := KubernetesCluster{}
	return &this
}

// GetEntities returns the Entities field value
// If the value is explicit nil, nil is returned
func (o *KubernetesCluster) GetEntities() *KubernetesClusterEntities {
	if o == nil {
		return nil
	}

	return o.Entities

}

// GetEntitiesOk returns a tuple with the Entities field value
// and a boolean to check if the value has been set.
// NOTE: If the value is an explicit nil, `nil, true` will be returned
func (o *KubernetesCluster) GetEntitiesOk() (*KubernetesClusterEntities, bool) {
	if o == nil {
		return nil, false
	}

	return o.Entities, true
}

// SetEntities sets field value
func (o *KubernetesCluster) SetEntities(v KubernetesClusterEntities) {

	o.Entities = &v

}

// HasEntities returns a boolean if a field has been set.
func (o *KubernetesCluster) HasEntities() bool {
	if o != nil && o.Entities != nil {
		return true
	}

	return false
}

// GetHref returns the Href field value
// If the value is explicit nil, nil is returned
func (o *KubernetesCluster) GetHref() *string {
	if o == nil {
		return nil
	}

	return o.Href

}

// GetHrefOk returns a tuple with the Href field value
// and a boolean to check if the value has been set.
// NOTE: If the value is an explicit nil, `nil, true` will be returned
func (o *KubernetesCluster) GetHrefOk() (*string, bool) {
	if o == nil {
		return nil, false
	}

	return o.Href, true
}

// SetHref sets field value
func (o *KubernetesCluster) SetHref(v string) {

	o.Href = &v

}

// HasHref returns a boolean if a field has been set.
func (o *KubernetesCluster) HasHref() bool {
	if o != nil && o.Href != nil {
		return true
	}

	return false
}

// GetId returns the Id field value
// If the value is explicit nil, nil is returned
func (o *KubernetesCluster) GetId() *string {
	if o == nil {
		return nil
	}

	return o.Id

}

// GetIdOk returns a tuple with the Id field value
// and a boolean to check if the value has been set.
// NOTE: If the value is an explicit nil, `nil, true` will be returned
func (o *KubernetesCluster) GetIdOk() (*string, bool) {
	if o == nil {
		return nil, false
	}

	return o.Id, true
}

// SetId sets field value
func (o *KubernetesCluster) SetId(v string) {

	o.Id = &v

}

// HasId returns a boolean if a field has been set.
func (o *KubernetesCluster) HasId() bool {
	if o != nil && o.Id != nil {
		return true
	}

	return false
}

// GetMetadata returns the Metadata field value
// If the value is explicit nil, nil is returned
func (o *KubernetesCluster) GetMetadata() *DatacenterElementMetadata {
	if o == nil {
		return nil
	}

	return o.Metadata

}

// GetMetadataOk returns a tuple with the Metadata field value
// and a boolean to check if the value has been set.
// NOTE: If the value is an explicit nil, `nil, true` will be returned
func (o *KubernetesCluster) GetMetadataOk() (*DatacenterElementMetadata, bool) {
	if o == nil {
		return nil, false
	}

	return o.Metadata, true
}

// SetMetadata sets field value
func (o *KubernetesCluster) SetMetadata(v DatacenterElementMetadata) {

	o.Metadata = &v

}

// HasMetadata returns a boolean if a field has been set.
func (o *KubernetesCluster) HasMetadata() bool {
	if o != nil && o.Metadata != nil {
		return true
	}

	return false
}

// GetProperties returns the Properties field value
// If the value is explicit nil, nil is returned
func (o *KubernetesCluster) GetProperties() *KubernetesClusterProperties {
	if o == nil {
		return nil
	}

	return o.Properties

}

// GetPropertiesOk returns a tuple with the Properties field value
// and a boolean to check if the value has been set.
// NOTE: If the value is an explicit nil, `nil, true` will be returned
func (o *KubernetesCluster) GetPropertiesOk() (*KubernetesClusterProperties, bool) {
	if o == nil {
		return nil, false
	}

	return o.Properties, true
}

// SetProperties sets field value
func (o *KubernetesCluster) SetProperties(v KubernetesClusterProperties) {

	o.Properties = &v

}

// HasProperties returns a boolean if a field has been set.
func (o *KubernetesCluster) HasProperties() bool {
	if o != nil && o.Properties != nil {
		return true
	}

	return false
}

// GetType returns the Type field value
// If the value is explicit nil, nil is returned
func (o *KubernetesCluster) GetType() *string {
	if o == nil {
		return nil
	}

	return o.Type

}

// GetTypeOk returns a tuple with the Type field value
// and a boolean to check if the value has been set.
// NOTE: If the value is an explicit nil, `nil, true` will be returned
func (o *KubernetesCluster) GetTypeOk() (*string, bool) {
	if o == nil {
		return nil, false
	}

	return o.Type, true
}

// SetType sets field value
func (o *KubernetesCluster) SetType(v string) {

	o.Type = &v

}

// HasType returns a boolean if a field has been set.
func (o *KubernetesCluster) HasType() bool {
	if o != nil && o.Type != nil {
		return true
	}

	return false
}

func (o KubernetesCluster) MarshalJSON() ([]byte, error) {
	toSerialize := map[string]interface{}{}
	if o.Entities != nil {
		toSerialize["entities"] = o.Entities
	}

	if o.Href != nil {
		toSerialize["href"] = o.Href
	}

	if o.Id != nil {
		toSerialize["id"] = o.Id
	}

	if o.Metadata != nil {
		toSerialize["metadata"] = o.Metadata
	}

	if o.Properties != nil {
		toSerialize["properties"] = o.Properties
	}

	if o.Type != nil {
		toSerialize["type"] = o.Type
	}

	return json.Marshal(toSerialize)
}

type NullableKubernetesCluster struct {
	value *KubernetesCluster
	isSet bool
}

func (v NullableKubernetesCluster) Get() *KubernetesCluster {
	return v.value
}

func (v *NullableKubernetesCluster) Set(val *KubernetesCluster) {
	v.value = val
	v.isSet = true
}

func (v NullableKubernetesCluster) IsSet() bool {
	return v.isSet
}

func (v *NullableKubernetesCluster) Unset() {
	v.value = nil
	v.isSet = false
}

func NewNullableKubernetesCluster(val *KubernetesCluster) *NullableKubernetesCluster {
	return &NullableKubernetesCluster{value: val, isSet: true}
}

func (v NullableKubernetesCluster) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.value)
}

func (v *NullableKubernetesCluster) UnmarshalJSON(src []byte) error {
	v.isSet = true
	return json.Unmarshal(src, &v.value)
}
