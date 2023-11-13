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

// GroupShareProperties struct for GroupShareProperties
type GroupShareProperties struct {
	// edit privilege on a resource
	EditPrivilege *bool `json:"editPrivilege,omitempty"`
	// share privilege on a resource
	SharePrivilege *bool `json:"sharePrivilege,omitempty"`
}

// NewGroupShareProperties instantiates a new GroupShareProperties object
// This constructor will assign default values to properties that have it defined,
// and makes sure properties required by API are set, but the set of arguments
// will change when the set of required properties is changed
func NewGroupShareProperties() *GroupShareProperties {
	this := GroupShareProperties{}

	return &this
}

// NewGroupSharePropertiesWithDefaults instantiates a new GroupShareProperties object
// This constructor will only assign default values to properties that have it defined,
// but it doesn't guarantee that properties required by API are set
func NewGroupSharePropertiesWithDefaults() *GroupShareProperties {
	this := GroupShareProperties{}
	return &this
}

// GetEditPrivilege returns the EditPrivilege field value
// If the value is explicit nil, nil is returned
func (o *GroupShareProperties) GetEditPrivilege() *bool {
	if o == nil {
		return nil
	}

	return o.EditPrivilege

}

// GetEditPrivilegeOk returns a tuple with the EditPrivilege field value
// and a boolean to check if the value has been set.
// NOTE: If the value is an explicit nil, `nil, true` will be returned
func (o *GroupShareProperties) GetEditPrivilegeOk() (*bool, bool) {
	if o == nil {
		return nil, false
	}

	return o.EditPrivilege, true
}

// SetEditPrivilege sets field value
func (o *GroupShareProperties) SetEditPrivilege(v bool) {

	o.EditPrivilege = &v

}

// HasEditPrivilege returns a boolean if a field has been set.
func (o *GroupShareProperties) HasEditPrivilege() bool {
	if o != nil && o.EditPrivilege != nil {
		return true
	}

	return false
}

// GetSharePrivilege returns the SharePrivilege field value
// If the value is explicit nil, nil is returned
func (o *GroupShareProperties) GetSharePrivilege() *bool {
	if o == nil {
		return nil
	}

	return o.SharePrivilege

}

// GetSharePrivilegeOk returns a tuple with the SharePrivilege field value
// and a boolean to check if the value has been set.
// NOTE: If the value is an explicit nil, `nil, true` will be returned
func (o *GroupShareProperties) GetSharePrivilegeOk() (*bool, bool) {
	if o == nil {
		return nil, false
	}

	return o.SharePrivilege, true
}

// SetSharePrivilege sets field value
func (o *GroupShareProperties) SetSharePrivilege(v bool) {

	o.SharePrivilege = &v

}

// HasSharePrivilege returns a boolean if a field has been set.
func (o *GroupShareProperties) HasSharePrivilege() bool {
	if o != nil && o.SharePrivilege != nil {
		return true
	}

	return false
}

func (o GroupShareProperties) MarshalJSON() ([]byte, error) {
	toSerialize := map[string]interface{}{}
	if o.EditPrivilege != nil {
		toSerialize["editPrivilege"] = o.EditPrivilege
	}

	if o.SharePrivilege != nil {
		toSerialize["sharePrivilege"] = o.SharePrivilege
	}

	return json.Marshal(toSerialize)
}

type NullableGroupShareProperties struct {
	value *GroupShareProperties
	isSet bool
}

func (v NullableGroupShareProperties) Get() *GroupShareProperties {
	return v.value
}

func (v *NullableGroupShareProperties) Set(val *GroupShareProperties) {
	v.value = val
	v.isSet = true
}

func (v NullableGroupShareProperties) IsSet() bool {
	return v.isSet
}

func (v *NullableGroupShareProperties) Unset() {
	v.value = nil
	v.isSet = false
}

func NewNullableGroupShareProperties(val *GroupShareProperties) *NullableGroupShareProperties {
	return &NullableGroupShareProperties{value: val, isSet: true}
}

func (v NullableGroupShareProperties) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.value)
}

func (v *NullableGroupShareProperties) UnmarshalJSON(src []byte) error {
	v.isSet = true
	return json.Unmarshal(src, &v.value)
}
