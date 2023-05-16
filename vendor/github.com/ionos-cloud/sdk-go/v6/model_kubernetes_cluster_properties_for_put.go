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

// KubernetesClusterPropertiesForPut struct for KubernetesClusterPropertiesForPut
type KubernetesClusterPropertiesForPut struct {
	// A Kubernetes cluster name. Valid Kubernetes cluster name must be 63 characters or less and must be empty or begin and end with an alphanumeric character ([a-z0-9A-Z]) with dashes (-), underscores (_), dots (.), and alphanumerics between.
	Name *string `json:"name"`
	// The Kubernetes version that the cluster is running. This limits which Kubernetes versions can run in a cluster's node pools. Also, not all Kubernetes versions are suitable upgrade targets for all earlier versions.
	K8sVersion        *string                      `json:"k8sVersion,omitempty"`
	MaintenanceWindow *KubernetesMaintenanceWindow `json:"maintenanceWindow,omitempty"`
	// Access to the K8s API server is restricted to these CIDRs. Intra-cluster traffic is not affected by this restriction. If no AllowList is specified, access is not limited. If an IP is specified without a subnet mask, the default value is 32 for IPv4 and 128 for IPv6.
	ApiSubnetAllowList *[]string `json:"apiSubnetAllowList,omitempty"`
	// List of S3 buckets configured for K8s usage. At the moment, it contains only one S3 bucket that is used to store K8s API audit logs.
	S3Buckets *[]S3Bucket `json:"s3Buckets,omitempty"`
}

// NewKubernetesClusterPropertiesForPut instantiates a new KubernetesClusterPropertiesForPut object
// This constructor will assign default values to properties that have it defined,
// and makes sure properties required by API are set, but the set of arguments
// will change when the set of required properties is changed
func NewKubernetesClusterPropertiesForPut(name string) *KubernetesClusterPropertiesForPut {
	this := KubernetesClusterPropertiesForPut{}

	this.Name = &name

	return &this
}

// NewKubernetesClusterPropertiesForPutWithDefaults instantiates a new KubernetesClusterPropertiesForPut object
// This constructor will only assign default values to properties that have it defined,
// but it doesn't guarantee that properties required by API are set
func NewKubernetesClusterPropertiesForPutWithDefaults() *KubernetesClusterPropertiesForPut {
	this := KubernetesClusterPropertiesForPut{}
	return &this
}

// GetName returns the Name field value
// If the value is explicit nil, the zero value for string will be returned
func (o *KubernetesClusterPropertiesForPut) GetName() *string {
	if o == nil {
		return nil
	}

	return o.Name

}

// GetNameOk returns a tuple with the Name field value
// and a boolean to check if the value has been set.
// NOTE: If the value is an explicit nil, `nil, true` will be returned
func (o *KubernetesClusterPropertiesForPut) GetNameOk() (*string, bool) {
	if o == nil {
		return nil, false
	}

	return o.Name, true
}

// SetName sets field value
func (o *KubernetesClusterPropertiesForPut) SetName(v string) {

	o.Name = &v

}

// HasName returns a boolean if a field has been set.
func (o *KubernetesClusterPropertiesForPut) HasName() bool {
	if o != nil && o.Name != nil {
		return true
	}

	return false
}

// GetK8sVersion returns the K8sVersion field value
// If the value is explicit nil, the zero value for string will be returned
func (o *KubernetesClusterPropertiesForPut) GetK8sVersion() *string {
	if o == nil {
		return nil
	}

	return o.K8sVersion

}

// GetK8sVersionOk returns a tuple with the K8sVersion field value
// and a boolean to check if the value has been set.
// NOTE: If the value is an explicit nil, `nil, true` will be returned
func (o *KubernetesClusterPropertiesForPut) GetK8sVersionOk() (*string, bool) {
	if o == nil {
		return nil, false
	}

	return o.K8sVersion, true
}

// SetK8sVersion sets field value
func (o *KubernetesClusterPropertiesForPut) SetK8sVersion(v string) {

	o.K8sVersion = &v

}

// HasK8sVersion returns a boolean if a field has been set.
func (o *KubernetesClusterPropertiesForPut) HasK8sVersion() bool {
	if o != nil && o.K8sVersion != nil {
		return true
	}

	return false
}

// GetMaintenanceWindow returns the MaintenanceWindow field value
// If the value is explicit nil, the zero value for KubernetesMaintenanceWindow will be returned
func (o *KubernetesClusterPropertiesForPut) GetMaintenanceWindow() *KubernetesMaintenanceWindow {
	if o == nil {
		return nil
	}

	return o.MaintenanceWindow

}

// GetMaintenanceWindowOk returns a tuple with the MaintenanceWindow field value
// and a boolean to check if the value has been set.
// NOTE: If the value is an explicit nil, `nil, true` will be returned
func (o *KubernetesClusterPropertiesForPut) GetMaintenanceWindowOk() (*KubernetesMaintenanceWindow, bool) {
	if o == nil {
		return nil, false
	}

	return o.MaintenanceWindow, true
}

// SetMaintenanceWindow sets field value
func (o *KubernetesClusterPropertiesForPut) SetMaintenanceWindow(v KubernetesMaintenanceWindow) {

	o.MaintenanceWindow = &v

}

// HasMaintenanceWindow returns a boolean if a field has been set.
func (o *KubernetesClusterPropertiesForPut) HasMaintenanceWindow() bool {
	if o != nil && o.MaintenanceWindow != nil {
		return true
	}

	return false
}

// GetApiSubnetAllowList returns the ApiSubnetAllowList field value
// If the value is explicit nil, the zero value for []string will be returned
func (o *KubernetesClusterPropertiesForPut) GetApiSubnetAllowList() *[]string {
	if o == nil {
		return nil
	}

	return o.ApiSubnetAllowList

}

// GetApiSubnetAllowListOk returns a tuple with the ApiSubnetAllowList field value
// and a boolean to check if the value has been set.
// NOTE: If the value is an explicit nil, `nil, true` will be returned
func (o *KubernetesClusterPropertiesForPut) GetApiSubnetAllowListOk() (*[]string, bool) {
	if o == nil {
		return nil, false
	}

	return o.ApiSubnetAllowList, true
}

// SetApiSubnetAllowList sets field value
func (o *KubernetesClusterPropertiesForPut) SetApiSubnetAllowList(v []string) {

	o.ApiSubnetAllowList = &v

}

// HasApiSubnetAllowList returns a boolean if a field has been set.
func (o *KubernetesClusterPropertiesForPut) HasApiSubnetAllowList() bool {
	if o != nil && o.ApiSubnetAllowList != nil {
		return true
	}

	return false
}

// GetS3Buckets returns the S3Buckets field value
// If the value is explicit nil, the zero value for []S3Bucket will be returned
func (o *KubernetesClusterPropertiesForPut) GetS3Buckets() *[]S3Bucket {
	if o == nil {
		return nil
	}

	return o.S3Buckets

}

// GetS3BucketsOk returns a tuple with the S3Buckets field value
// and a boolean to check if the value has been set.
// NOTE: If the value is an explicit nil, `nil, true` will be returned
func (o *KubernetesClusterPropertiesForPut) GetS3BucketsOk() (*[]S3Bucket, bool) {
	if o == nil {
		return nil, false
	}

	return o.S3Buckets, true
}

// SetS3Buckets sets field value
func (o *KubernetesClusterPropertiesForPut) SetS3Buckets(v []S3Bucket) {

	o.S3Buckets = &v

}

// HasS3Buckets returns a boolean if a field has been set.
func (o *KubernetesClusterPropertiesForPut) HasS3Buckets() bool {
	if o != nil && o.S3Buckets != nil {
		return true
	}

	return false
}

func (o KubernetesClusterPropertiesForPut) MarshalJSON() ([]byte, error) {
	toSerialize := map[string]interface{}{}
	if o.Name != nil {
		toSerialize["name"] = o.Name
	}
	if o.K8sVersion != nil {
		toSerialize["k8sVersion"] = o.K8sVersion
	}
	if o.MaintenanceWindow != nil {
		toSerialize["maintenanceWindow"] = o.MaintenanceWindow
	}
	if o.ApiSubnetAllowList != nil {
		toSerialize["apiSubnetAllowList"] = o.ApiSubnetAllowList
	}
	if o.S3Buckets != nil {
		toSerialize["s3Buckets"] = o.S3Buckets
	}
	return json.Marshal(toSerialize)
}

type NullableKubernetesClusterPropertiesForPut struct {
	value *KubernetesClusterPropertiesForPut
	isSet bool
}

func (v NullableKubernetesClusterPropertiesForPut) Get() *KubernetesClusterPropertiesForPut {
	return v.value
}

func (v *NullableKubernetesClusterPropertiesForPut) Set(val *KubernetesClusterPropertiesForPut) {
	v.value = val
	v.isSet = true
}

func (v NullableKubernetesClusterPropertiesForPut) IsSet() bool {
	return v.isSet
}

func (v *NullableKubernetesClusterPropertiesForPut) Unset() {
	v.value = nil
	v.isSet = false
}

func NewNullableKubernetesClusterPropertiesForPut(val *KubernetesClusterPropertiesForPut) *NullableKubernetesClusterPropertiesForPut {
	return &NullableKubernetesClusterPropertiesForPut{value: val, isSet: true}
}

func (v NullableKubernetesClusterPropertiesForPut) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.value)
}

func (v *NullableKubernetesClusterPropertiesForPut) UnmarshalJSON(src []byte) error {
	v.isSet = true
	return json.Unmarshal(src, &v.value)
}
