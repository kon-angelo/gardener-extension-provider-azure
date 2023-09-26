//  Copyright (c) 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
//  Licensed under the Apache License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License.
//  You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//  See the License for the specific language governing permissions and
//  limitations under the License.

package infraflow

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v4"

	"github.com/gardener/gardener-extension-provider-azure/pkg/azure/client"
)

// EnrichResponseWithUserManagedIPs adds the IDs of user managed IPs to the input map of associated IPs of the NATs
func (f *FlowContext) EnrichResponseWithUserManagedIPs(ctx context.Context, res map[string][]*armnetwork.PublicIPAddress) error {
	ips := f.tf.UserManagedIPs()
	if len(ips) == 0 {
		return nil
	}
	c, err := f.factory.PublicIP()
	if err != nil {
		return err
	}
	for _, ip := range ips {
		resp, err := c.Get(ctx, ip.ResourceGroup, ip.Name, nil)
		if err == nil {
			res[ip.SubnetName] = append(res[ip.SubnetName], &armnetwork.PublicIPAddress{
				ID: resp.ID,
			})
		} else {
			return err
		}
	}
	return nil
}

func (f *FlowContext) createOrUpdateNatGateway(ctx context.Context, nat zoneTf, ips map[string][]*armnetwork.PublicIPAddress, client client.NatGateway) (*armnetwork.NatGateway, error) {
	params := armnetwork.NatGateway{
		Properties: &armnetwork.NatGatewayPropertiesFormat{
			IdleTimeoutInMinutes: nat.idleConnectionTimeoutMinutes,
		},
		Location: to.Ptr(f.tf.Region()),
		SKU:      &armnetwork.NatGatewaySKU{Name: to.Ptr(armnetwork.NatGatewaySKUNameStandard)},
	}
	ipResources, ok := ips[nat.SubnetName()]
	if !ok {
		return nil, fmt.Errorf("no public IP found for NAT Gateway %s", nat.NatName())
	} else {
		params.Properties.PublicIPAddresses = []*armnetwork.SubResource{}
		for _, ip := range ipResources {
			params.Properties.PublicIPAddresses = append(params.Properties.PublicIPAddresses, &armnetwork.SubResource{ID: ip.ID})
		}
	}
	if nat.Zone() != nil {
		params.Zones = []*string{nat.Zone()}
	}
	resp, err := client.CreateOrUpdate(ctx, f.tf.ResourceGroup(), nat.NatName(), params)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// EnsureSubnets creates or updates subnets
func (f *FlowContext) EnsureSubnetsOld(ctx context.Context, securityGroup armnetwork.SecurityGroup, routeTable armnetwork.RouteTable, nats map[string]*armnetwork.NatGateway) (err error) {
	subnetClient, err := f.factory.Subnet()
	if err != nil {
		return err
	}

	subnets := f.tf.Zones()
	for _, subnet := range subnets {
		endpoints := make([]*armnetwork.ServiceEndpointPropertiesFormat, 0)
		for _, endpoint := range subnet.serviceEndpoints {
			endpoints = append(endpoints, &armnetwork.ServiceEndpointPropertiesFormat{
				Service: to.Ptr(endpoint),
			})
		}

		parameters := armnetwork.Subnet{
			Properties: &armnetwork.SubnetPropertiesFormat{
				AddressPrefix:    to.Ptr(subnet.cidr),
				ServiceEndpoints: endpoints,
				NetworkSecurityGroup: &armnetwork.SecurityGroup{
					ID: securityGroup.ID,
				},
				RouteTable: &armnetwork.RouteTable{
					ID: routeTable.ID,
				},
			},
		}
		nat, ok := nats[subnet.SubnetName()]
		if ok {
			parameters.Properties.NatGateway = &armnetwork.SubResource{
				ID: nat.ID,
			}
		}

		vnetRgroup := f.tf.Vnet().ResourceGroup() // try to use existing vnet resource
		if vnetRgroup == nil {
			vnetRgroup = to.Ptr(f.tf.ResourceGroup()) // expect that it was created previously
		}
		_, err = subnetClient.CreateOrUpdate(ctx, *vnetRgroup, f.tf.Vnet().Name(), subnet.SubnetName(), parameters)
	}
	return err
}
