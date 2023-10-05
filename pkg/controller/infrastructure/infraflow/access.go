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
	"errors"
	"reflect"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v4"
	"github.com/hashicorp/go-azure-helpers/resourcemanager/resourceids"

	"github.com/gardener/gardener-extension-provider-azure/pkg/azure/client"
)

var _ Access = &access{}

// Access provides additional methods that are build on top of the azure client primitives.
type Access interface {
	DeletePublicIP(context.Context, string, string) error
	DisassociatePublicIP(ctx context.Context, rgName, natName, pipID string) error
	DeleteNatGateway(ctx context.Context, rgName, natName string) error
	DisassociateNatGateway(ctx context.Context, rgName, natName string) error
}

type access struct {
	f client.Factory
}

func (p *access) DeletePublicIP(ctx context.Context, rgName, pipName string) error {
	pipClient, err := p.f.PublicIP()
	if err != nil {
		return err
	}

	pip, err := pipClient.Get(ctx, rgName, pipName, to.Ptr("natGateway"))
	if err != nil {
		return err
	}

	if pip.Properties.NatGateway != nil && pip.Properties.NatGateway.Name != nil {
		err := p.DisassociatePublicIP(ctx, rgName, *pip.Properties.NatGateway.Name, *pip.ID)
		if err != nil {
			return err
		}
	}

	return pipClient.Delete(ctx, rgName, pipName)
}

// DisassociatePublicIP disassociates a publicIP from it's attached NAT Gateway.
func (p *access) DisassociatePublicIP(ctx context.Context, rgName, natName, pipID string) error {
	nc, err := p.f.NatGateway()
	if err != nil {
		return err
	}

	nat, err := nc.Get(ctx, rgName, natName, nil)
	var natPips []*armnetwork.SubResource
	for _, natPip := range nat.Properties.PublicIPAddresses {
		if natPip != nil && !reflect.DeepEqual(*natPip.ID, pipID) {
			natPips = append(natPips, natPip)
		}

	}
	nat.Properties.PublicIPAddresses = natPips

	_, err = nc.CreateOrUpdate(ctx, rgName, natName, *nat)
	return err
}

func (p *access) DeleteNatGateway(ctx context.Context, rgName, natName string) error {
	nc, err := p.f.NatGateway()
	if err != nil {
		return err
	}
	err = p.DisassociateNatGateway(ctx, rgName, natName)
	if err != nil {
		return err
	}
	return nc.Delete(ctx, rgName, natName)
}

// DisassociateNatGateway disassociates a NAT Gateway from all the subnets.
func (p *access) DisassociateNatGateway(ctx context.Context, rgName, natName string) error {
	nc, err := p.f.NatGateway()
	if err != nil {
		return err
	}
	sc, err := p.f.Subnet()
	if err != nil {
		return err
	}

	nat, err := nc.Get(ctx, rgName, natName, to.Ptr("subnets"))
	if err != nil {
		return err
	}

	var joinErr error
	for _, subnetId := range nat.Properties.Subnets {
		if subnetId == nil || subnetId.ID == nil {
			continue
		}
		azId, err := resourceids.ParseAzureResourceID(*subnetId.ID)
		vnetName := azId.Path["virtualNetworks"]
		subnetName := azId.Path["subnets"]
		subnet, err := sc.Get(ctx, azId.ResourceGroup, vnetName, subnetName, nil)
		if err != nil {
			return err
		}
		if subnet == nil {
			continue
		}
		subnet.Properties.NatGateway = nil
		_, err = sc.CreateOrUpdate(ctx, rgName, vnetName, subnetName, *subnet)
		joinErr = errors.Join(joinErr, err)

	}

	return joinErr
}

// PublicIPAddress is an alias for the real pip type.
type PublicIPAddress armnetwork.PublicIPAddress

// MustDelete public current and target spec between PIPs and decide if they need to be deleted.
func (p PublicIPAddress) MustDelete(target armnetwork.PublicIPAddress) bool {
	if !reflect.DeepEqual(p.Location, target.Location) {
		return true
	}
	if !reflect.DeepEqual(p.Zones, target.Zones) {
		return true
	}
	if !reflect.DeepEqual(p.Properties.PublicIPAllocationMethod, target.Properties.PublicIPAllocationMethod) {
		return true
	}
	return false
}

type NatGatewayCheck armnetwork.NatGateway

func (n *NatGatewayCheck) MustDelete(target armnetwork.NatGateway) bool {
	if !reflect.DeepEqual(n.Location, target.Location) {
		return true
	}
	if !reflect.DeepEqual(n.Zones, target.Zones) {
		return true
	}

	return false
}
