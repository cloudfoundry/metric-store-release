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

// ApplicationLoadBalancerHttpRule struct for ApplicationLoadBalancerHttpRule
type ApplicationLoadBalancerHttpRule struct {
	// The unique name of the Application Load Balancer HTTP rule.
	Name *string `json:"name"`
	// The HTTP rule type.
	Type *string `json:"type"`
	// The ID of the target group; this parameter is mandatory and is valid only for 'FORWARD' actions.
	TargetGroup *string `json:"targetGroup,omitempty"`
	// Indicates whether the query part of the URI should be dropped and is valid only for 'REDIRECT' actions. Default value is 'FALSE', the redirect URI does not contain any query parameters.
	DropQuery *bool `json:"dropQuery,omitempty"`
	// The location for the redirection; this parameter is mandatory and valid only for 'REDIRECT' actions.
	Location *string `json:"location,omitempty"`
	// The status code is for 'REDIRECT' and 'STATIC' actions only.   If the HTTP rule is 'REDIRECT' the valid values are: 301, 302, 303, 307, 308; default value is '301'.  If the HTTP rule is 'STATIC' the valid values are from the range 200-599; default value is '503'.
	StatusCode *int32 `json:"statusCode,omitempty"`
	// The response message of the request; this parameter is mandatory for 'STATIC' actions.
	ResponseMessage *string `json:"responseMessage,omitempty"`
	// Specifies the content type and is valid only for 'STATIC' actions.
	ContentType *string `json:"contentType,omitempty"`
	// An array of items in the collection. The action will be executed only if each condition is met; the rule will always be applied if no conditions are set.
	Conditions *[]ApplicationLoadBalancerHttpRuleCondition `json:"conditions,omitempty"`
}

// NewApplicationLoadBalancerHttpRule instantiates a new ApplicationLoadBalancerHttpRule object
// This constructor will assign default values to properties that have it defined,
// and makes sure properties required by API are set, but the set of arguments
// will change when the set of required properties is changed
func NewApplicationLoadBalancerHttpRule(name string, type_ string) *ApplicationLoadBalancerHttpRule {
	this := ApplicationLoadBalancerHttpRule{}

	this.Name = &name
	this.Type = &type_

	return &this
}

// NewApplicationLoadBalancerHttpRuleWithDefaults instantiates a new ApplicationLoadBalancerHttpRule object
// This constructor will only assign default values to properties that have it defined,
// but it doesn't guarantee that properties required by API are set
func NewApplicationLoadBalancerHttpRuleWithDefaults() *ApplicationLoadBalancerHttpRule {
	this := ApplicationLoadBalancerHttpRule{}
	return &this
}

// GetName returns the Name field value
// If the value is explicit nil, the zero value for string will be returned
func (o *ApplicationLoadBalancerHttpRule) GetName() *string {
	if o == nil {
		return nil
	}

	return o.Name

}

// GetNameOk returns a tuple with the Name field value
// and a boolean to check if the value has been set.
// NOTE: If the value is an explicit nil, `nil, true` will be returned
func (o *ApplicationLoadBalancerHttpRule) GetNameOk() (*string, bool) {
	if o == nil {
		return nil, false
	}

	return o.Name, true
}

// SetName sets field value
func (o *ApplicationLoadBalancerHttpRule) SetName(v string) {

	o.Name = &v

}

// HasName returns a boolean if a field has been set.
func (o *ApplicationLoadBalancerHttpRule) HasName() bool {
	if o != nil && o.Name != nil {
		return true
	}

	return false
}

// GetType returns the Type field value
// If the value is explicit nil, the zero value for string will be returned
func (o *ApplicationLoadBalancerHttpRule) GetType() *string {
	if o == nil {
		return nil
	}

	return o.Type

}

// GetTypeOk returns a tuple with the Type field value
// and a boolean to check if the value has been set.
// NOTE: If the value is an explicit nil, `nil, true` will be returned
func (o *ApplicationLoadBalancerHttpRule) GetTypeOk() (*string, bool) {
	if o == nil {
		return nil, false
	}

	return o.Type, true
}

// SetType sets field value
func (o *ApplicationLoadBalancerHttpRule) SetType(v string) {

	o.Type = &v

}

// HasType returns a boolean if a field has been set.
func (o *ApplicationLoadBalancerHttpRule) HasType() bool {
	if o != nil && o.Type != nil {
		return true
	}

	return false
}

// GetTargetGroup returns the TargetGroup field value
// If the value is explicit nil, the zero value for string will be returned
func (o *ApplicationLoadBalancerHttpRule) GetTargetGroup() *string {
	if o == nil {
		return nil
	}

	return o.TargetGroup

}

// GetTargetGroupOk returns a tuple with the TargetGroup field value
// and a boolean to check if the value has been set.
// NOTE: If the value is an explicit nil, `nil, true` will be returned
func (o *ApplicationLoadBalancerHttpRule) GetTargetGroupOk() (*string, bool) {
	if o == nil {
		return nil, false
	}

	return o.TargetGroup, true
}

// SetTargetGroup sets field value
func (o *ApplicationLoadBalancerHttpRule) SetTargetGroup(v string) {

	o.TargetGroup = &v

}

// HasTargetGroup returns a boolean if a field has been set.
func (o *ApplicationLoadBalancerHttpRule) HasTargetGroup() bool {
	if o != nil && o.TargetGroup != nil {
		return true
	}

	return false
}

// GetDropQuery returns the DropQuery field value
// If the value is explicit nil, the zero value for bool will be returned
func (o *ApplicationLoadBalancerHttpRule) GetDropQuery() *bool {
	if o == nil {
		return nil
	}

	return o.DropQuery

}

// GetDropQueryOk returns a tuple with the DropQuery field value
// and a boolean to check if the value has been set.
// NOTE: If the value is an explicit nil, `nil, true` will be returned
func (o *ApplicationLoadBalancerHttpRule) GetDropQueryOk() (*bool, bool) {
	if o == nil {
		return nil, false
	}

	return o.DropQuery, true
}

// SetDropQuery sets field value
func (o *ApplicationLoadBalancerHttpRule) SetDropQuery(v bool) {

	o.DropQuery = &v

}

// HasDropQuery returns a boolean if a field has been set.
func (o *ApplicationLoadBalancerHttpRule) HasDropQuery() bool {
	if o != nil && o.DropQuery != nil {
		return true
	}

	return false
}

// GetLocation returns the Location field value
// If the value is explicit nil, the zero value for string will be returned
func (o *ApplicationLoadBalancerHttpRule) GetLocation() *string {
	if o == nil {
		return nil
	}

	return o.Location

}

// GetLocationOk returns a tuple with the Location field value
// and a boolean to check if the value has been set.
// NOTE: If the value is an explicit nil, `nil, true` will be returned
func (o *ApplicationLoadBalancerHttpRule) GetLocationOk() (*string, bool) {
	if o == nil {
		return nil, false
	}

	return o.Location, true
}

// SetLocation sets field value
func (o *ApplicationLoadBalancerHttpRule) SetLocation(v string) {

	o.Location = &v

}

// HasLocation returns a boolean if a field has been set.
func (o *ApplicationLoadBalancerHttpRule) HasLocation() bool {
	if o != nil && o.Location != nil {
		return true
	}

	return false
}

// GetStatusCode returns the StatusCode field value
// If the value is explicit nil, the zero value for int32 will be returned
func (o *ApplicationLoadBalancerHttpRule) GetStatusCode() *int32 {
	if o == nil {
		return nil
	}

	return o.StatusCode

}

// GetStatusCodeOk returns a tuple with the StatusCode field value
// and a boolean to check if the value has been set.
// NOTE: If the value is an explicit nil, `nil, true` will be returned
func (o *ApplicationLoadBalancerHttpRule) GetStatusCodeOk() (*int32, bool) {
	if o == nil {
		return nil, false
	}

	return o.StatusCode, true
}

// SetStatusCode sets field value
func (o *ApplicationLoadBalancerHttpRule) SetStatusCode(v int32) {

	o.StatusCode = &v

}

// HasStatusCode returns a boolean if a field has been set.
func (o *ApplicationLoadBalancerHttpRule) HasStatusCode() bool {
	if o != nil && o.StatusCode != nil {
		return true
	}

	return false
}

// GetResponseMessage returns the ResponseMessage field value
// If the value is explicit nil, the zero value for string will be returned
func (o *ApplicationLoadBalancerHttpRule) GetResponseMessage() *string {
	if o == nil {
		return nil
	}

	return o.ResponseMessage

}

// GetResponseMessageOk returns a tuple with the ResponseMessage field value
// and a boolean to check if the value has been set.
// NOTE: If the value is an explicit nil, `nil, true` will be returned
func (o *ApplicationLoadBalancerHttpRule) GetResponseMessageOk() (*string, bool) {
	if o == nil {
		return nil, false
	}

	return o.ResponseMessage, true
}

// SetResponseMessage sets field value
func (o *ApplicationLoadBalancerHttpRule) SetResponseMessage(v string) {

	o.ResponseMessage = &v

}

// HasResponseMessage returns a boolean if a field has been set.
func (o *ApplicationLoadBalancerHttpRule) HasResponseMessage() bool {
	if o != nil && o.ResponseMessage != nil {
		return true
	}

	return false
}

// GetContentType returns the ContentType field value
// If the value is explicit nil, the zero value for string will be returned
func (o *ApplicationLoadBalancerHttpRule) GetContentType() *string {
	if o == nil {
		return nil
	}

	return o.ContentType

}

// GetContentTypeOk returns a tuple with the ContentType field value
// and a boolean to check if the value has been set.
// NOTE: If the value is an explicit nil, `nil, true` will be returned
func (o *ApplicationLoadBalancerHttpRule) GetContentTypeOk() (*string, bool) {
	if o == nil {
		return nil, false
	}

	return o.ContentType, true
}

// SetContentType sets field value
func (o *ApplicationLoadBalancerHttpRule) SetContentType(v string) {

	o.ContentType = &v

}

// HasContentType returns a boolean if a field has been set.
func (o *ApplicationLoadBalancerHttpRule) HasContentType() bool {
	if o != nil && o.ContentType != nil {
		return true
	}

	return false
}

// GetConditions returns the Conditions field value
// If the value is explicit nil, the zero value for []ApplicationLoadBalancerHttpRuleCondition will be returned
func (o *ApplicationLoadBalancerHttpRule) GetConditions() *[]ApplicationLoadBalancerHttpRuleCondition {
	if o == nil {
		return nil
	}

	return o.Conditions

}

// GetConditionsOk returns a tuple with the Conditions field value
// and a boolean to check if the value has been set.
// NOTE: If the value is an explicit nil, `nil, true` will be returned
func (o *ApplicationLoadBalancerHttpRule) GetConditionsOk() (*[]ApplicationLoadBalancerHttpRuleCondition, bool) {
	if o == nil {
		return nil, false
	}

	return o.Conditions, true
}

// SetConditions sets field value
func (o *ApplicationLoadBalancerHttpRule) SetConditions(v []ApplicationLoadBalancerHttpRuleCondition) {

	o.Conditions = &v

}

// HasConditions returns a boolean if a field has been set.
func (o *ApplicationLoadBalancerHttpRule) HasConditions() bool {
	if o != nil && o.Conditions != nil {
		return true
	}

	return false
}

func (o ApplicationLoadBalancerHttpRule) MarshalJSON() ([]byte, error) {
	toSerialize := map[string]interface{}{}
	if o.Name != nil {
		toSerialize["name"] = o.Name
	}
	if o.Type != nil {
		toSerialize["type"] = o.Type
	}
	if o.TargetGroup != nil {
		toSerialize["targetGroup"] = o.TargetGroup
	}
	if o.DropQuery != nil {
		toSerialize["dropQuery"] = o.DropQuery
	}
	if o.Location != nil {
		toSerialize["location"] = o.Location
	}
	if o.StatusCode != nil {
		toSerialize["statusCode"] = o.StatusCode
	}
	if o.ResponseMessage != nil {
		toSerialize["responseMessage"] = o.ResponseMessage
	}
	if o.ContentType != nil {
		toSerialize["contentType"] = o.ContentType
	}
	if o.Conditions != nil {
		toSerialize["conditions"] = o.Conditions
	}
	return json.Marshal(toSerialize)
}

type NullableApplicationLoadBalancerHttpRule struct {
	value *ApplicationLoadBalancerHttpRule
	isSet bool
}

func (v NullableApplicationLoadBalancerHttpRule) Get() *ApplicationLoadBalancerHttpRule {
	return v.value
}

func (v *NullableApplicationLoadBalancerHttpRule) Set(val *ApplicationLoadBalancerHttpRule) {
	v.value = val
	v.isSet = true
}

func (v NullableApplicationLoadBalancerHttpRule) IsSet() bool {
	return v.isSet
}

func (v *NullableApplicationLoadBalancerHttpRule) Unset() {
	v.value = nil
	v.isSet = false
}

func NewNullableApplicationLoadBalancerHttpRule(val *ApplicationLoadBalancerHttpRule) *NullableApplicationLoadBalancerHttpRule {
	return &NullableApplicationLoadBalancerHttpRule{value: val, isSet: true}
}

func (v NullableApplicationLoadBalancerHttpRule) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.value)
}

func (v *NullableApplicationLoadBalancerHttpRule) UnmarshalJSON(src []byte) error {
	v.isSet = true
	return json.Unmarshal(src, &v.value)
}
