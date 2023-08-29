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
		resp, err := c.Get(ctx, ip.ResourceGroup, ip.Name)
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

func checkAllZonesWithFn(name string, zones []zoneTf, check func(zone zoneTf, name string) bool) bool {
	for _, n := range zones {
		if check(n, name) {
			return true
		}
	}
	return false
}

// EnsureNatGateways creates or updates NAT Gateways. It also deletes old NATGateways.
func (f *FlowContext) EnsureNatGateways(ctx context.Context, ips map[string][]*armnetwork.PublicIPAddress) (map[string]*armnetwork.NatGateway, error) {
	res := make(map[string]*armnetwork.NatGateway)
	c, err := f.factory.NatGateway()
	if err != nil {
		return res, err
	}
	err = f.deleteOldNatGateways(ctx, c)
	if err != nil {
		return res, err
	}
	for _, nat := range f.tf.EnabledNats() {
		resp, err := f.createOrUpdateNatGateway(ctx, nat, ips, c)
		if err != nil {
			return res, err
		}
		res[nat.SubnetName()] = resp
	}
	return res, nil
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

// delete NAT Gateways that got disabled
func (f *FlowContext) deleteOldNatGateways(ctx context.Context, client client.NatGateway) error {
	existingNats, err := client.List(ctx, f.tf.ResourceGroup())
	if err != nil {
		return err
	}
	for _, nat := range existingNats {
		if nat.Name == nil {
			continue
		}
		isNatInNats := checkAllZonesWithFn(*nat.Name, f.tf.EnabledNats(), func(nat zoneTf, name string) bool { return nat.NatName() == name })
		if !isNatInNats {
			err := client.Delete(ctx, f.tf.ResourceGroup(), *nat.Name)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// EnsureSubnets creates or updates subnets
func (f *FlowContext) EnsureSubnets(ctx context.Context, securityGroup armnetwork.SecurityGroup, routeTable armnetwork.RouteTable, nats map[string]*armnetwork.NatGateway) (err error) {
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

// EnsurePublicIPs creates or updates PublicIPs for the NATs
func (f *FlowContext) EnsurePublicIPs(ctx context.Context) (map[string][]*armnetwork.PublicIPAddress, error) {
	res := make(map[string][]*armnetwork.PublicIPAddress)
	c, err := f.factory.PublicIP()
	if err != nil {
		return res, err
	}

	err = f.deleteOldNatIPs(ctx, c)
	if err != nil {
		return res, err
	}
	ips := f.tf.EnabledNats()
	if len(ips) == 0 {
		return res, nil
	}
	for _, ip := range ips {
		params := armnetwork.PublicIPAddress{
			Location: to.Ptr(f.tf.Region()),
			Properties: &armnetwork.PublicIPAddressPropertiesFormat{
				PublicIPAllocationMethod: to.Ptr(armnetwork.IPAllocationMethodStatic),
			},
			SKU:   &armnetwork.PublicIPAddressSKU{Name: to.Ptr(armnetwork.PublicIPAddressSKUNameStandard)},
			Zones: []*string{},
		}
		if ip.Zone() != nil {
			params.Zones = []*string{ip.Zone()}
		}
		resp, err := c.CreateOrUpdate(ctx, f.tf.ResourceGroup(), ip.IpName(), params)
		if err != nil {
			return res, err
		}
		res[ip.SubnetName()] = append(res[ip.SubnetName()], resp)

	}
	return res, nil
}

// delete IPs of NAT Gateways that got disabled
func (f *FlowContext) deleteOldNatIPs(ctx context.Context, client client.PublicIP) error {
	existingIPs, err := client.List(ctx, f.tf.ResourceGroup())
	if err != nil {
		return err
	}
	for _, ip := range existingIPs {
		if ip.Name == nil {
			continue
		}
		isIpInNats := checkAllZonesWithFn(*ip.Name, f.tf.EnabledNats(), func(nat zoneTf, name string) bool { return nat.IpName() == name })
		if !isIpInNats {
			err := client.Delete(ctx, f.tf.ResourceGroup(), *ip.Name)
			if err != nil {
				return err
			}
		}
	}
	return nil
}
