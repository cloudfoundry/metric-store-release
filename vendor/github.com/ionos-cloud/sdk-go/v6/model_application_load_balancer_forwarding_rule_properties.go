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

// ApplicationLoadBalancerForwardingRuleProperties struct for ApplicationLoadBalancerForwardingRuleProperties
type ApplicationLoadBalancerForwardingRuleProperties struct {
	// The maximum time in milliseconds to wait for the client to acknowledge or send data; default is 50,000 (50 seconds).
	ClientTimeout *int32 `json:"clientTimeout,omitempty"`
	// An array of items in the collection. The original order of rules is preserved during processing, except that rules of the 'FORWARD' type are processed after the rules with other defined actions. The relative order of the 'FORWARD' type rules is also preserved during the processing.
	HttpRules *[]ApplicationLoadBalancerHttpRule `json:"httpRules,omitempty"`
	// The listening (inbound) IP.
	ListenerIp *string `json:"listenerIp"`
	// The listening (inbound) port number; the valid range is 1 to 65535.
	ListenerPort *int32 `json:"listenerPort"`
	// The name of the Application Load Balancer forwarding rule.
	Name *string `json:"name"`
	// The balancing protocol.
	Protocol *string `json:"protocol"`
	// Array of items in the collection.
	ServerCertificates *[]string `json:"serverCertificates,omitempty"`
}

// NewApplicationLoadBalancerForwardingRuleProperties instantiates a new ApplicationLoadBalancerForwardingRuleProperties object
// This constructor will assign default values to properties that have it defined,
// and makes sure properties required by API are set, but the set of arguments
// will change when the set of required properties is changed
func NewApplicationLoadBalancerForwardingRuleProperties(listenerIp string, listenerPort int32, name string, protocol string) *ApplicationLoadBalancerForwardingRuleProperties {
	this := ApplicationLoadBalancerForwardingRuleProperties{}

	this.ListenerIp = &listenerIp
	this.ListenerPort = &listenerPort
	this.Name = &name
	this.Protocol = &protocol

	return &this
}

// NewApplicationLoadBalancerForwardingRulePropertiesWithDefaults instantiates a new ApplicationLoadBalancerForwardingRuleProperties object
// This constructor will only assign default values to properties that have it defined,
// but it doesn't guarantee that properties required by API are set
func NewApplicationLoadBalancerForwardingRulePropertiesWithDefaults() *ApplicationLoadBalancerForwardingRuleProperties {
	this := ApplicationLoadBalancerForwardingRuleProperties{}
	return &this
}

// GetClientTimeout returns the ClientTimeout field value
// If the value is explicit nil, nil is returned
func (o *ApplicationLoadBalancerForwardingRuleProperties) GetClientTimeout() *int32 {
	if o == nil {
		return nil
	}

	return o.ClientTimeout

}

// GetClientTimeoutOk returns a tuple with the ClientTimeout field value
// and a boolean to check if the value has been set.
// NOTE: If the value is an explicit nil, `nil, true` will be returned
func (o *ApplicationLoadBalancerForwardingRuleProperties) GetClientTimeoutOk() (*int32, bool) {
	if o == nil {
		return nil, false
	}

	return o.ClientTimeout, true
}

// SetClientTimeout sets field value
func (o *ApplicationLoadBalancerForwardingRuleProperties) SetClientTimeout(v int32) {

	o.ClientTimeout = &v

}

// HasClientTimeout returns a boolean if a field has been set.
func (o *ApplicationLoadBalancerForwardingRuleProperties) HasClientTimeout() bool {
	if o != nil && o.ClientTimeout != nil {
		return true
	}

	return false
}

// GetHttpRules returns the HttpRules field value
// If the value is explicit nil, nil is returned
func (o *ApplicationLoadBalancerForwardingRuleProperties) GetHttpRules() *[]ApplicationLoadBalancerHttpRule {
	if o == nil {
		return nil
	}

	return o.HttpRules

}

// GetHttpRulesOk returns a tuple with the HttpRules field value
// and a boolean to check if the value has been set.
// NOTE: If the value is an explicit nil, `nil, true` will be returned
func (o *ApplicationLoadBalancerForwardingRuleProperties) GetHttpRulesOk() (*[]ApplicationLoadBalancerHttpRule, bool) {
	if o == nil {
		return nil, false
	}

	return o.HttpRules, true
}

// SetHttpRules sets field value
func (o *ApplicationLoadBalancerForwardingRuleProperties) SetHttpRules(v []ApplicationLoadBalancerHttpRule) {

	o.HttpRules = &v

}

// HasHttpRules returns a boolean if a field has been set.
func (o *ApplicationLoadBalancerForwardingRuleProperties) HasHttpRules() bool {
	if o != nil && o.HttpRules != nil {
		return true
	}

	return false
}

// GetListenerIp returns the ListenerIp field value
// If the value is explicit nil, nil is returned
func (o *ApplicationLoadBalancerForwardingRuleProperties) GetListenerIp() *string {
	if o == nil {
		return nil
	}

	return o.ListenerIp

}

// GetListenerIpOk returns a tuple with the ListenerIp field value
// and a boolean to check if the value has been set.
// NOTE: If the value is an explicit nil, `nil, true` will be returned
func (o *ApplicationLoadBalancerForwardingRuleProperties) GetListenerIpOk() (*string, bool) {
	if o == nil {
		return nil, false
	}

	return o.ListenerIp, true
}

// SetListenerIp sets field value
func (o *ApplicationLoadBalancerForwardingRuleProperties) SetListenerIp(v string) {

	o.ListenerIp = &v

}

// HasListenerIp returns a boolean if a field has been set.
func (o *ApplicationLoadBalancerForwardingRuleProperties) HasListenerIp() bool {
	if o != nil && o.ListenerIp != nil {
		return true
	}

	return false
}

// GetListenerPort returns the ListenerPort field value
// If the value is explicit nil, nil is returned
func (o *ApplicationLoadBalancerForwardingRuleProperties) GetListenerPort() *int32 {
	if o == nil {
		return nil
	}

	return o.ListenerPort

}

// GetListenerPortOk returns a tuple with the ListenerPort field value
// and a boolean to check if the value has been set.
// NOTE: If the value is an explicit nil, `nil, true` will be returned
func (o *ApplicationLoadBalancerForwardingRuleProperties) GetListenerPortOk() (*int32, bool) {
	if o == nil {
		return nil, false
	}

	return o.ListenerPort, true
}

// SetListenerPort sets field value
func (o *ApplicationLoadBalancerForwardingRuleProperties) SetListenerPort(v int32) {

	o.ListenerPort = &v

}

// HasListenerPort returns a boolean if a field has been set.
func (o *ApplicationLoadBalancerForwardingRuleProperties) HasListenerPort() bool {
	if o != nil && o.ListenerPort != nil {
		return true
	}

	return false
}

// GetName returns the Name field value
// If the value is explicit nil, nil is returned
func (o *ApplicationLoadBalancerForwardingRuleProperties) GetName() *string {
	if o == nil {
		return nil
	}

	return o.Name

}

// GetNameOk returns a tuple with the Name field value
// and a boolean to check if the value has been set.
// NOTE: If the value is an explicit nil, `nil, true` will be returned
func (o *ApplicationLoadBalancerForwardingRuleProperties) GetNameOk() (*string, bool) {
	if o == nil {
		return nil, false
	}

	return o.Name, true
}

// SetName sets field value
func (o *ApplicationLoadBalancerForwardingRuleProperties) SetName(v string) {

	o.Name = &v

}

// HasName returns a boolean if a field has been set.
func (o *ApplicationLoadBalancerForwardingRuleProperties) HasName() bool {
	if o != nil && o.Name != nil {
		return true
	}

	return false
}

// GetProtocol returns the Protocol field value
// If the value is explicit nil, nil is returned
func (o *ApplicationLoadBalancerForwardingRuleProperties) GetProtocol() *string {
	if o == nil {
		return nil
	}

	return o.Protocol

}

// GetProtocolOk returns a tuple with the Protocol field value
// and a boolean to check if the value has been set.
// NOTE: If the value is an explicit nil, `nil, true` will be returned
func (o *ApplicationLoadBalancerForwardingRuleProperties) GetProtocolOk() (*string, bool) {
	if o == nil {
		return nil, false
	}

	return o.Protocol, true
}

// SetProtocol sets field value
func (o *ApplicationLoadBalancerForwardingRuleProperties) SetProtocol(v string) {

	o.Protocol = &v

}

// HasProtocol returns a boolean if a field has been set.
func (o *ApplicationLoadBalancerForwardingRuleProperties) HasProtocol() bool {
	if o != nil && o.Protocol != nil {
		return true
	}

	return false
}

// GetServerCertificates returns the ServerCertificates field value
// If the value is explicit nil, nil is returned
func (o *ApplicationLoadBalancerForwardingRuleProperties) GetServerCertificates() *[]string {
	if o == nil {
		return nil
	}

	return o.ServerCertificates

}

// GetServerCertificatesOk returns a tuple with the ServerCertificates field value
// and a boolean to check if the value has been set.
// NOTE: If the value is an explicit nil, `nil, true` will be returned
func (o *ApplicationLoadBalancerForwardingRuleProperties) GetServerCertificatesOk() (*[]string, bool) {
	if o == nil {
		return nil, false
	}

	return o.ServerCertificates, true
}

// SetServerCertificates sets field value
func (o *ApplicationLoadBalancerForwardingRuleProperties) SetServerCertificates(v []string) {

	o.ServerCertificates = &v

}

// HasServerCertificates returns a boolean if a field has been set.
func (o *ApplicationLoadBalancerForwardingRuleProperties) HasServerCertificates() bool {
	if o != nil && o.ServerCertificates != nil {
		return true
	}

	return false
}

func (o ApplicationLoadBalancerForwardingRuleProperties) MarshalJSON() ([]byte, error) {
	toSerialize := map[string]interface{}{}
	if o.ClientTimeout != nil {
		toSerialize["clientTimeout"] = o.ClientTimeout
	}

	if o.HttpRules != nil {
		toSerialize["httpRules"] = o.HttpRules
	}

	if o.ListenerIp != nil {
		toSerialize["listenerIp"] = o.ListenerIp
	}

	if o.ListenerPort != nil {
		toSerialize["listenerPort"] = o.ListenerPort
	}

	if o.Name != nil {
		toSerialize["name"] = o.Name
	}

	if o.Protocol != nil {
		toSerialize["protocol"] = o.Protocol
	}

	if o.ServerCertificates != nil {
		toSerialize["serverCertificates"] = o.ServerCertificates
	}

	return json.Marshal(toSerialize)
}

type NullableApplicationLoadBalancerForwardingRuleProperties struct {
	value *ApplicationLoadBalancerForwardingRuleProperties
	isSet bool
}

func (v NullableApplicationLoadBalancerForwardingRuleProperties) Get() *ApplicationLoadBalancerForwardingRuleProperties {
	return v.value
}

func (v *NullableApplicationLoadBalancerForwardingRuleProperties) Set(val *ApplicationLoadBalancerForwardingRuleProperties) {
	v.value = val
	v.isSet = true
}

func (v NullableApplicationLoadBalancerForwardingRuleProperties) IsSet() bool {
	return v.isSet
}

func (v *NullableApplicationLoadBalancerForwardingRuleProperties) Unset() {
	v.value = nil
	v.isSet = false
}

func NewNullableApplicationLoadBalancerForwardingRuleProperties(val *ApplicationLoadBalancerForwardingRuleProperties) *NullableApplicationLoadBalancerForwardingRuleProperties {
	return &NullableApplicationLoadBalancerForwardingRuleProperties{value: val, isSet: true}
}

func (v NullableApplicationLoadBalancerForwardingRuleProperties) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.value)
}

func (v *NullableApplicationLoadBalancerForwardingRuleProperties) UnmarshalJSON(src []byte) error {
	v.isSet = true
	return json.Unmarshal(src, &v.value)
}
