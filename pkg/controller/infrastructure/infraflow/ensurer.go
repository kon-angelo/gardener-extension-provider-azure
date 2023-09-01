// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package infraflow

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v5"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v4"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/pointer"

	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/v1alpha1"
)

// EnsureResourceGroup creates or updates the resource group
func (f *FlowContext) EnsureResourceGroup(ctx context.Context) error {
	rgClient, err := f.factory.Group()
	if err != nil {
		return err
	}

	rg := armresources.ResourceGroup{
		Location: to.Ptr(f.infra.Spec.Region),
	}

	_, err = rgClient.CreateOrUpdate(ctx, f.adapter.ResourceGroupName(), rg)
	return err
}

func (f *FlowContext) EnsureVnet(ctx context.Context) error {
	if f.adapter.GardenerVnet() {
		return f.ensureGardenerVnet(ctx)
	}

	return f.ensureUserVnet(ctx)
}

// EnsureVnet creates or updates a Vnet
func (f *FlowContext) ensureGardenerVnet(ctx context.Context) error {
	azId := AzureResourceIdentifier{
		ResourceGroup: f.adapter.ResourceGroupName(),
		Name:          f.adapter.VnetName(),
		Kind:          VirtualNetwork,
	}

	c, err := f.factory.Vnet()
	if err != nil {
		return err
	}

	vnet, err := c.Get(ctx, azId.ResourceGroup, azId.Name)
	if err != nil {
		return err
	}

	if vnet != nil {
		if pointer.StringDeref(vnet.Location, "") != f.adapter.Region() {
			return NewTerminalSpecMismatch(azId, "Location", f.adapter.Region())
		}
	}

	if vnet == nil {
		vnet = &armnetwork.VirtualNetwork{}
	}

	vnet = f.applyTargetVnet(vnet)
	_, err = c.CreateOrUpdate(ctx, azId.ResourceGroup, azId.Name, *vnet)
	return err
}

func (f *FlowContext) ensureUserVnet(ctx context.Context) error {
	azId := AzureResourceIdentifier{
		ResourceGroup: f.adapter.ResourceGroupName(),
		Name:          f.adapter.VnetName(),
		Kind:          VirtualNetwork,
	}

	c, err := f.factory.Vnet()
	if err != nil {
		return err
	}

	vnet, err := c.Get(ctx, f.adapter.VnetResourceGroup(), f.adapter.VnetName())
	if err != nil {
		return err
	}

	if vnet == nil {
		return NewTerminalConditionError(azId, fmt.Errorf("user vnet not found"))
	}

	return nil
}

// EnsureAvailabilitySet creates or updates an AvailabilitySet
func (f *FlowContext) EnsureAvailabilitySet(ctx context.Context) error {
	log := f.LogFromContext(ctx)
	ok, err := f.adapter.AvailabilitySetRequired()
	if err != nil {
		return err
	}
	if !ok {
		log.Info("skipping ensuring availability set")
		return nil
	}

	asClient, err := f.factory.AvailabilitySet()
	if err != nil {
		return err
	}
	avsetParams, err := f.adapter.AvailabilitySet()
	if err != nil {
		return err
	}
	parameters := armcompute.AvailabilitySet{
		Location: to.Ptr(f.adapter.Region()),
		// the DomainCounts are computed from the current InfrastructureStatus. They cannot be updated after shoot creation.
		Properties: &armcompute.AvailabilitySetProperties{
			PlatformFaultDomainCount:  avsetParams.CountFaultDomains,
			PlatformUpdateDomainCount: avsetParams.CountUpdateDomains,
		},
		SKU: &armcompute.SKU{Name: to.Ptr(string(armcompute.AvailabilitySetSKUTypesAligned))}, // equal to managed = True in tf
	}
	_, err = asClient.CreateOrUpdate(ctx, f.tf.ResourceGroup(), avsetParams.Name, parameters)
	return err
}

// EnsureRouteTable creates or updates the route table
func (f *FlowContext) EnsureRouteTable(ctx context.Context) error {
	routeTable, err := f.ensureRouteTable(ctx)
	f.whiteboard.Set(routeTableID, *routeTable.ID)
	return err
}

// EnsureRouteTables creates or updates a RouteTable
func (f *FlowContext) ensureRouteTable(ctx context.Context) (*armnetwork.RouteTable, error) {
	azId := AzureResourceIdentifier{
		ResourceGroup: f.adapter.ResourceGroupName(),
		Name:          f.adapter.RouteTableName(),
		Kind:          RouteTable,
	}
	c, err := f.factory.RouteTables()
	if err != nil {
		return nil, err
	}

	rt, err := c.Get(ctx, azId.ResourceGroup, azId.Name)
	if err != nil {
		return nil, err
	}

	if rt != nil {
		// if the location doesn't match, attempt to delete the route table.
		if pointer.StringDeref(rt.Location, "") != f.adapter.Region() {
			if pointer.StringDeref(rt.Location, "") != f.adapter.Region() {
				return nil, NewTerminalSpecMismatch(azId, "Location", f.adapter.Region())
			}
		}
	}

	// create the RT
	if rt == nil {
		parameters := armnetwork.RouteTable{
			Location:   to.Ptr(f.tf.Region()),
			Properties: &armnetwork.RouteTablePropertiesFormat{},
		}

		return c.CreateOrUpdate(ctx, f.tf.ResourceGroup(), f.tf.RouteTableName(), parameters)
	}

	return rt, nil
}

// EnsureSecurityGroup creates or updates a SecurityGroup
func (f *FlowContext) EnsureSecurityGroup(ctx context.Context) error {
	sg, err := f.ensureSecurityGroup(ctx)
	if err != nil {
		return err
	}

	f.whiteboard.Set(sGroupID, *sg.ID)
	return nil
}

func (f *FlowContext) ensureSecurityGroup(ctx context.Context) (*armnetwork.SecurityGroup, error) {
	azId := AzureResourceIdentifier{
		ResourceGroup: f.adapter.ResourceGroupName(),
		Name:          f.adapter.RouteTableName(),
		Kind:          SecurityGroup,
	}
	c, err := f.factory.NetworkSecurityGroup()
	if err != nil {
		return nil, err
	}

	nsg, err := c.Get(ctx, azId.ResourceGroup, azId.Name)
	if err != nil {
		return nil, err
	}

	if nsg != nil {
		// if the location doesn't match, attempt to delete the NSG.
		if pointer.StringDeref(nsg.Location, "") != f.tf.Region() {
			err := c.Delete(ctx, f.tf.ResourceGroup(), f.tf.RouteTableName())
			if err != nil {
				return nil, err
			}
			nsg = nil
		}
	}

	// create the NSG if it not there
	if nsg == nil {
		parameters := armnetwork.SecurityGroup{
			Location:   to.Ptr(f.tf.Region()),
			Properties: &armnetwork.SecurityGroupPropertiesFormat{},
		}
		return c.CreateOrUpdate(ctx, f.tf.ResourceGroup(), f.tf.SecurityGroupName(), parameters)
	}

	return nsg, nil
}

// EnsurePublicIPs2 creates or updates PublicIPs for the NATs
func (f *FlowContext) EnsurePublicIPs2(ctx context.Context) error {
	var (
		log       = f.LogFromContext(ctx)
		joinError error
	)

	c, err := f.factory.PublicIP()
	if err != nil {
		return err
	}

	var (
		toDelete sets.Set[AzureResourceIdentifier]
		// toDelete    []*armnetwork.PublicIPAddress
		toReconcile = map[string]armnetwork.PublicIPAddress{}
	)

	targetNats := f.tf.EnabledNats()
	for _, nat := range targetNats {
		// for user IPs we can only check if they exist.
		if len(nat.UserManagedIP()) > 0 {
			for _, uip := range nat.UserManagedIP() {
				actualIP, err := c.Get(ctx, uip.ResourceGroup, uip.Name, nil)
				if err != nil {
					joinError = errors.Join(joinError, err)
				}
				if actualIP == nil {
					joinError = errors.Join(joinError, fmt.Errorf("failed to locate user IP: %s, %s", uip.ResourceGroup, uip.Name))
				}
			}
		} else {
			pip := armnetwork.PublicIPAddress{
				Location: to.Ptr(f.tf.Region()),
				Properties: &armnetwork.PublicIPAddressPropertiesFormat{
					PublicIPAllocationMethod: to.Ptr(armnetwork.IPAllocationMethodStatic),
				},
				SKU:   &armnetwork.PublicIPAddressSKU{Name: to.Ptr(armnetwork.PublicIPAddressSKUNameStandard)},
				Zones: []*string{},
			}
			if nat.Zone() != nil {
				pip.Zones = []*string{nat.Zone()}
			}
			toReconcile[nat.IpName()] = pip
		}
	}
	if joinError != nil {
		return joinError
	}

	currentIPs, err := c.List(ctx, f.tf.ResourceGroup())
	if err != nil {
		return err
	}

	// filter only these IPs prefixed by the cluster name.
	currentIPs = Filter(currentIPs, func(address *armnetwork.PublicIPAddress) bool {
		return address.Name != nil && strings.HasPrefix(*address.Name, f.tf.ClusterName())
	})

	for _, currentIP := range currentIPs {
		if currentIP.Name == nil {
			continue
		}
		id := AzureResourceIdentifier{
			ResourceGroup: f.tf.ResourceGroup(),
			Name:          *currentIP.Name,
		}
		// delete all the resources that are not in the list of target resources
		if _, ok := toReconcile[*currentIP.Name]; !ok {
			toDelete.Insert(id)
			continue
		}

		// delete all resources who spec cannot be updated to match target spec.
		targetIP := toReconcile[*currentIP.Name]
		if PublicIPAddress(*currentIP).Compare(PublicIPAddress(targetIP)) {
			toDelete.Insert(id)
			continue
		}
	}

	for _, ip := range toDelete.UnsortedList() {
		log.Info("deleting IP", ip.Name)
		err := f.provider.DeletePublicIP2(ctx, ip.ResourceGroup, ip.Name)
		if err != nil {
			joinError = errors.Join(joinError, err)
		}
	}
	if joinError != nil {
		return joinError
	}

	for ipName, ip := range toReconcile {
		_, err := c.CreateOrUpdate(ctx, f.tf.ResourceGroup(), ipName, ip)
		if err != nil {
			joinError = errors.Join(joinError, err)
		}
	}
	return joinError
}

// EnsureSubnets2 creates or updates subnets
func (f *FlowContext) EnsureSubnets2(ctx context.Context, securityGroup armnetwork.SecurityGroup, routeTable armnetwork.RouteTable, _ map[string]*armnetwork.NatGateway) (err error) {
	var joinError error
	c, err := f.factory.Subnet()
	if err != nil {
		return err
	}

	vnetRgroup := f.tf.Vnet().ResourceGroup() // try to use existing vnet resource
	if vnetRgroup == nil {
		vnetRgroup = to.Ptr(f.tf.ResourceGroup()) // expect that it was created previously
	}
	vnetName := f.tf.Vnet().Name()

	currentSubnets, err := c.List(ctx, *vnetRgroup, vnetName)
	if err != nil {
		return err
	}

	filteredSubnets := Filter(currentSubnets, func(s *armnetwork.Subnet) bool {
		return s != nil && s.Name != nil && strings.HasPrefix(*s.Name, f.tf.ClusterName())
	})

	subnetsMap := ToMap(filteredSubnets, func(subnet *armnetwork.Subnet) string {
		if subnet == nil || subnet.Name == nil {
			return ""
		}
		return *subnet.Name
	})

	zones := f.tf.Zones()
	var (
		toReconcile map[string]*armnetwork.Subnet
		toDelete    []armnetwork.Subnet
	)

	for _, zone := range zones {
		subnet := &armnetwork.Subnet{}
		if _, ok := subnetsMap[zone.SubnetName()]; ok {
			subnet = subnetsMap[zone.SubnetName()]
		}

		endpoints := make([]*armnetwork.ServiceEndpointPropertiesFormat, 0)
		for _, endpoint := range zone.serviceEndpoints {
			endpoints = append(endpoints, &armnetwork.ServiceEndpointPropertiesFormat{
				Service: to.Ptr(endpoint),
			})
		}

		if subnet.Properties == nil {
			subnet.Properties = &armnetwork.SubnetPropertiesFormat{}
		}

		subnet.Properties.AddressPrefixes = []*string{to.Ptr(zone.cidr)}
		subnet.Properties.NetworkSecurityGroup = &armnetwork.SecurityGroup{
			ID: securityGroup.ID,
		}
		subnet.Properties.RouteTable = &armnetwork.RouteTable{
			ID: routeTable.ID,
		}

		toReconcile[zone.SubnetName()] = subnet
	}

	for _, s := range currentSubnets {
		if _, ok := toReconcile[*s.Name]; !ok {
			toDelete = append(toDelete, *s)
		}
	}

	for _, s := range toDelete {
		rid, err := AzureResourceIdentifierFromID(*s.ID)
		if err != nil {
			joinError = errors.Join(joinError, err)
			continue
		}
		if err := c.Delete(ctx, rid.ResourceGroup, *vnetRgroup, rid.Name); err != nil {
			joinError = errors.Join(joinError, err)
			continue
		}
	}
	if joinError != nil {
		return joinError
	}

	for subnetName, subnet := range toReconcile {
		_, err = c.CreateOrUpdate(ctx, *vnetRgroup, vnetName, subnetName, *subnet)
		if err != nil {
			joinError = errors.Join(joinError, err)
			continue
		}
	}
	return joinError
}

// func (f *FlowContext) EnsureNATGateways2(ctx context.Context) error {
// 	var (
// 		log = f.LogFromContext(ctx)
// 	)
//
// 	c, err := f.factory.NatGateway()
// 	if err != nil {
// 		return err
// 	}
//
// 	currentNats, err := c.List(ctx, f.tf.ResourceGroupName())
// 	if err != nil {
// 		return err
// 	}
//
// 	filteredNats := Filter(currentNats, func(s *armnetwork.NatGateway) bool {
// 		return s != nil && s.Name != nil && strings.HasPrefix(*s.Name, f.tf.ClusterName())
// 	})
//
// 	filteredNatsMap := ToMap(filteredNats, func(gateway *armnetwork.NatGateway) string {
// 		if gateway == nil {
// 			return ""
// 		}
//
// 		return pointer.StringDeref(gateway.Name, "")
// 	})
//
// 	var (
// 		_ []*armnetwork.NatGateway
// 		_ map[string]armnetwork.NatGateway
// 	)
//
// 	nats := f.tf.EnabledNats()
// 	for _, targetNat := range nats {
// 		nat := &armnetwork.NatGateway{}
// 		if found, ok := filteredNatsMap[targetNat.NatName()]; ok {
// 			nat = found
// 		}
//
// 		if nat.Properties == nil {
// 			nat.Properties = &armnetwork.NatGatewayPropertiesFormat{}
// 		}
// 		nat.Properties.IdleTimeoutInMinutes = targetNat.idleConnectionTimeoutMinutes
// 		nat.Properties.PublicIPAddresses = []*armnetwork.SubResource{}
// 	}
// }

// GetInfrastructureStatus returns the infrastructure status
func (f *FlowContext) GetInfrastructureStatus(ctx context.Context) (*v1alpha1.InfrastructureStatus, error) {
	status := f.tf.StaticInfrastructureStatus()
	err := f.enrichStatusWithIdentity(ctx, status)
	if err != nil {
		return status, err
	}
	err = f.enrichStatusWithAvailabilitySet(ctx, status)
	if err != nil {
		return status, err
	}
	return status, nil
}

func (f *FlowContext) GetInfrastructureState() (*runtime.RawExtension, error) {
	json, err := NewPersistentState().ToJSON()
	if err != nil {
		return nil, err
	}

	return &runtime.RawExtension{
		Raw: json,
	}, nil
}

func (f *FlowContext) enrichStatusWithAvailabilitySet(ctx context.Context, status *v1alpha1.InfrastructureStatus) error {
	if f.tf.isCreate(AvailabilitySet) {
		c, err := f.factory.AvailabilitySet()
		if err != nil {
			return err
		}
		avset := f.tf.AvailabilitySet()
		res, err := c.Get(ctx, f.tf.ResourceGroup(), avset.Name)
		if err != nil {
			return err
		}
		status.AvailabilitySets = append(status.AvailabilitySets, v1alpha1.AvailabilitySet{
			Name:               avset.Name,
			ID:                 *res.ID,
			CountFaultDomains:  pointer.Int32(avset.CountFaultDomains),
			CountUpdateDomains: pointer.Int32(avset.CountUpdateDomains),
			Purpose:            v1alpha1.PurposeNodes,
		})
	}
	return nil
}

func (f *FlowContext) enrichStatusWithIdentity(ctx context.Context, status *v1alpha1.InfrastructureStatus) error {
	if identity := f.tf.Identity(); identity != nil {
		c, err := f.factory.ManagedUserIdentity()
		if err != nil {
			return err
		}
		res, err := c.Get(ctx, identity.ResourceGroup, identity.Name)
		if err != nil {
			return err
		}
		if res.ID == nil || res.ClientID == nil {
			return nil
		}

		status.Identity = &v1alpha1.IdentityStatus{
			ID:       *res.ID,
			ClientID: res.ClientID.String(),
		}
	}
	return nil
}

func (f *FlowContext) DeleteResourceGroup(ctx context.Context) error {
	c, err := f.factory.Group()
	if err != nil {
		return err
	}
	return c.Delete(ctx, f.tf.ResourceGroup())
}

func (f *FlowContext) applyTargetVnet(v *armnetwork.VirtualNetwork) *armnetwork.VirtualNetwork {
	if v.Properties == nil {
		v.Properties = &armnetwork.VirtualNetworkPropertiesFormat{}
	}

	v.Location = to.Ptr(f.infra.Spec.Region)
	v.Properties.AddressSpace = &armnetwork.AddressSpace{
		AddressPrefixes: []*string{f.tf.Vnet().Cidr()},
	}

	if ddosId := f.cfg.Networks.VNet.DDosProtectionPlanID; ddosId != nil {
		v.Properties.EnableDdosProtection = to.Ptr(true)
		v.Properties.DdosProtectionPlan = &armnetwork.SubResource{ID: ddosId}
	} else {
		v.Properties.DdosProtectionPlan = nil
		v.Properties.EnableDdosProtection = to.Ptr(false)
	}

	return v
}

// deleteSubnetsInForeignGroup deletes all managed subnets in a foreign resource group
func (f *FlowContext) deleteSubnetsInForeignGroup(ctx context.Context) error {
	if !f.tf.isCreate(Vnet) {
		subnetClient, err := f.factory.Subnet()
		if err != nil {
			return err
		}
		subnets := f.tf.Zones()
		for _, subnet := range subnets {
			resourceGroup := *f.tf.Vnet().ResourceGroup() // safe because we manage a foreign vnet
			err := subnetClient.Delete(ctx, resourceGroup, f.tf.Vnet().Name(), subnet.SubnetName())
			if err != nil {
				return err
			}
		}
		if err != nil {
			return fmt.Errorf("failed to delete foreign subnet: %w", err)
		}
	}
	return nil
}
