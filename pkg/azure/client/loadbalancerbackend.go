// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v4"

	"github.com/gardener/gardener-extension-provider-azure/pkg/internal"
)

// LoadBalancersClient implements the interface for the LoadBalancers client.
type LoadBalancersBackendAddressPoolClient struct {
	client *armnetwork.LoadBalancerBackendAddressPoolsClient
}

func NewLoadBalancersBackendAddressPoolClient(auth internal.ClientAuth, tc azcore.TokenCredential, opts *arm.ClientOptions) (*LoadBalancersBackendAddressPoolClient, error) {
	client, err := armnetwork.NewLoadBalancerBackendAddressPoolsClient(auth.SubscriptionID, tc, opts)
	return &LoadBalancersBackendAddressPoolClient{client}, err
}

// Get gets a given virtual load balancer by name
func (c *LoadBalancersBackendAddressPoolClient) Get(ctx context.Context, resourceGroupName, lbName string, name string) (*armnetwork.BackendAddressPool, error) {
	res, err := c.client.Get(ctx, resourceGroupName, lbName, name, nil)
	if err != nil {
		return nil, FilterNotFoundError(err)
	}
	return &res.BackendAddressPool, err
}

// List lists all subnets of a given virtual network.
func (c *LoadBalancersBackendAddressPoolClient) List(ctx context.Context, resourceGroupName, lbName string) ([]*armnetwork.BackendAddressPool, error) {
	pager := c.client.NewListPager(resourceGroupName, lbName, nil)
	var bp []*armnetwork.BackendAddressPool
	for pager.More() {
		page, err := pager.NextPage(ctx)
		bp = append(bp, page.Value...)
		if err != nil {
			return nil, err
		}
	}
	return bp, nil
}

// Delete deletes a subnet in a given virtual network.
func (c *LoadBalancersBackendAddressPoolClient) Delete(ctx context.Context, resourceGroupName, loadBalancerName string) error {
	poller, err := c.client.BeginDelete(ctx, resourceGroupName, loadBalancerName, nil)
	if err != nil {
		return FilterNotFoundError(err)
	}

	_, err = poller.PollUntilDone(ctx, nil)
	return err
}

// CreateOrUpdate creates or updates a load balancer.
func (c *LoadBalancersBackendAddressPoolClient) CreateOrUpdate(ctx context.Context, resourceGroupName string, name string, parameters armnetwork.LoadBalancerBackendAddressPoolsClient) (*armnetwork.LoadBalancer, error) {
	poller, err := c.client.BeginCreateOrUpdate(ctx, resourceGroupName, name, parameters, nil)
	if err != nil {
		return nil, fmt.Errorf("cannot create loadbalancer: %v", err)
	}
	res, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("cannot create loadbalancer: %v", err)
	}
	return &res.LoadBalancer, err
}
