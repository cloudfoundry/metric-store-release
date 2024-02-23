// Code generated by ifacemaker; DO NOT EDIT.

package hcloud

import (
	"context"
	"net"
)

// ILoadBalancerClient ...
type ILoadBalancerClient interface {
	// GetByID retrieves a Load Balancer by its ID. If the Load Balancer does not exist, nil is returned.
	GetByID(ctx context.Context, id int64) (*LoadBalancer, *Response, error)
	// GetByName retrieves a Load Balancer by its name. If the Load Balancer does not exist, nil is returned.
	GetByName(ctx context.Context, name string) (*LoadBalancer, *Response, error)
	// Get retrieves a Load Balancer by its ID if the input can be parsed as an integer, otherwise it
	// retrieves a Load Balancer by its name. If the Load Balancer does not exist, nil is returned.
	Get(ctx context.Context, idOrName string) (*LoadBalancer, *Response, error)
	// List returns a list of Load Balancers for a specific page.
	//
	// Please note that filters specified in opts are not taken into account
	// when their value corresponds to their zero value or when they are empty.
	List(ctx context.Context, opts LoadBalancerListOpts) ([]*LoadBalancer, *Response, error)
	// All returns all Load Balancers.
	All(ctx context.Context) ([]*LoadBalancer, error)
	// AllWithOpts returns all Load Balancers for the given options.
	AllWithOpts(ctx context.Context, opts LoadBalancerListOpts) ([]*LoadBalancer, error)
	// Update updates a Load Balancer.
	Update(ctx context.Context, loadBalancer *LoadBalancer, opts LoadBalancerUpdateOpts) (*LoadBalancer, *Response, error)
	// Create creates a new Load Balancer.
	Create(ctx context.Context, opts LoadBalancerCreateOpts) (LoadBalancerCreateResult, *Response, error)
	// Delete deletes a Load Balancer.
	Delete(ctx context.Context, loadBalancer *LoadBalancer) (*Response, error)
	// AddServerTarget adds a server target to a Load Balancer.
	AddServerTarget(ctx context.Context, loadBalancer *LoadBalancer, opts LoadBalancerAddServerTargetOpts) (*Action, *Response, error)
	// RemoveServerTarget removes a server target from a Load Balancer.
	RemoveServerTarget(ctx context.Context, loadBalancer *LoadBalancer, server *Server) (*Action, *Response, error)
	// AddLabelSelectorTarget adds a label selector target to a Load Balancer.
	AddLabelSelectorTarget(ctx context.Context, loadBalancer *LoadBalancer, opts LoadBalancerAddLabelSelectorTargetOpts) (*Action, *Response, error)
	// RemoveLabelSelectorTarget removes a label selector target from a Load Balancer.
	RemoveLabelSelectorTarget(ctx context.Context, loadBalancer *LoadBalancer, labelSelector string) (*Action, *Response, error)
	// AddIPTarget adds an IP target to a Load Balancer.
	AddIPTarget(ctx context.Context, loadBalancer *LoadBalancer, opts LoadBalancerAddIPTargetOpts) (*Action, *Response, error)
	// RemoveIPTarget removes an IP target from a Load Balancer.
	RemoveIPTarget(ctx context.Context, loadBalancer *LoadBalancer, ip net.IP) (*Action, *Response, error)
	// AddService adds a service to a Load Balancer.
	AddService(ctx context.Context, loadBalancer *LoadBalancer, opts LoadBalancerAddServiceOpts) (*Action, *Response, error)
	// UpdateService updates a Load Balancer service.
	UpdateService(ctx context.Context, loadBalancer *LoadBalancer, listenPort int, opts LoadBalancerUpdateServiceOpts) (*Action, *Response, error)
	// DeleteService deletes a Load Balancer service.
	DeleteService(ctx context.Context, loadBalancer *LoadBalancer, listenPort int) (*Action, *Response, error)
	// ChangeProtection changes the resource protection level of a Load Balancer.
	ChangeProtection(ctx context.Context, loadBalancer *LoadBalancer, opts LoadBalancerChangeProtectionOpts) (*Action, *Response, error)
	// ChangeAlgorithm changes the algorithm of a Load Balancer.
	ChangeAlgorithm(ctx context.Context, loadBalancer *LoadBalancer, opts LoadBalancerChangeAlgorithmOpts) (*Action, *Response, error)
	// AttachToNetwork attaches a Load Balancer to a network.
	AttachToNetwork(ctx context.Context, loadBalancer *LoadBalancer, opts LoadBalancerAttachToNetworkOpts) (*Action, *Response, error)
	// DetachFromNetwork detaches a Load Balancer from a network.
	DetachFromNetwork(ctx context.Context, loadBalancer *LoadBalancer, opts LoadBalancerDetachFromNetworkOpts) (*Action, *Response, error)
	// EnablePublicInterface enables the Load Balancer's public network interface.
	EnablePublicInterface(ctx context.Context, loadBalancer *LoadBalancer) (*Action, *Response, error)
	// DisablePublicInterface disables the Load Balancer's public network interface.
	DisablePublicInterface(ctx context.Context, loadBalancer *LoadBalancer) (*Action, *Response, error)
	// ChangeType changes a Load Balancer's type.
	ChangeType(ctx context.Context, loadBalancer *LoadBalancer, opts LoadBalancerChangeTypeOpts) (*Action, *Response, error)
	// GetMetrics obtains metrics for a Load Balancer.
	GetMetrics(ctx context.Context, lb *LoadBalancer, opts LoadBalancerGetMetricsOpts) (*LoadBalancerMetrics, *Response, error)
	// ChangeDNSPtr changes or resets the reverse DNS pointer for a Load Balancer.
	// Pass a nil ptr to reset the reverse DNS pointer to its default value.
	ChangeDNSPtr(ctx context.Context, lb *LoadBalancer, ip string, ptr *string) (*Action, *Response, error)
}
