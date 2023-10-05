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
	"reflect"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v5"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v4"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/pointer"

	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/v1alpha1"
)

// Key names for the whiteboard object to pass results between the reconcilation tasks
const (
	routeTableId     = "route_table_id"
	securityGroupId  = "security_group_id"
	natGatewayMapKey = "nategateway_map"
	publicIPMapKey   = "public-ips"
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
	_, err = asClient.CreateOrUpdate(ctx, f.adapter.ResourceGroupName(), f.adapter.AvailabilitySetName(), parameters)
	return err
}

// EnsureRouteTable creates or updates the route table
func (f *FlowContext) EnsureRouteTable(ctx context.Context) error {
	routeTable, err := f.ensureRouteTable(ctx)
	f.whiteboard.Set(routeTableId, *routeTable.ID)
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
		if pointer.StringDeref(rt.Location, "") != f.adapter.Region() {
			return nil, NewTerminalSpecMismatch(azId, "Location", f.adapter.Region())
		}
	}

	// create the RT
	if rt == nil {
		parameters := armnetwork.RouteTable{
			Location:   to.Ptr(f.tf.Region()),
			Properties: &armnetwork.RouteTablePropertiesFormat{},
		}

		return c.CreateOrUpdate(ctx, f.adapter.ResourceGroupName(), f.adapter.RouteTableName(), parameters)
	}

	return rt, nil
}

// EnsureSecurityGroup creates or updates a SecurityGroup
func (f *FlowContext) EnsureSecurityGroup(ctx context.Context) error {
	sg, err := f.ensureSecurityGroup(ctx)
	if err != nil {
		return err
	}

	f.whiteboard.Set(securityGroupId, *sg.ID)
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
		if pointer.StringDeref(nsg.Location, "") != f.adapter.Region() {
			return nil, NewTerminalSpecMismatch(azId, "Location", f.adapter.Region())
		}
	}

	// create the NSG if it not there
	if nsg == nil {
		parameters := armnetwork.SecurityGroup{
			Location:   to.Ptr(f.adapter.Region()),
			Properties: &armnetwork.SecurityGroupPropertiesFormat{},
		}
		return c.CreateOrUpdate(ctx, f.adapter.ResourceGroupName(), f.adapter.SecurityGroupName(), parameters)
	}

	return nsg, nil
}

func (f *FlowContext) EnsurePublicIPs(ctx context.Context) error {
	ipMap, err := f.ensurePublicIPs(ctx)
	userIpMap, usrErr := f.ensureUserPublicIps(ctx)

	ipMap = Join(ipMap, userIpMap)
	f.whiteboard.SetObject(publicIPMapKey, ipMap)
	return errors.Join(err, usrErr)
}

func (f *FlowContext) ensureUserPublicIps(ctx context.Context) (map[AzureResourceIdentifier]string, error) {
	var (
		joinError error
		result    map[AzureResourceIdentifier]string
	)

	c, err := f.factory.PublicIP()
	if err != nil {
		return nil, err
	}

	for _, ipFromConfig := range f.adapter.IPs() {
		if !ipFromConfig.userManaged {
			continue
		}

		userIP, err := c.Get(ctx, ipFromConfig.ResourceGroup, ipFromConfig.Name, nil)
		if err != nil {
			joinError = errors.Join(joinError, err)
		} else if userIP == nil {
			joinError = errors.Join(joinError, fmt.Errorf(fmt.Sprintf("failed to locate user IP: %s, %s", "", "")))
		} else {
			result[ipFromConfig.AzureResourceIdentifier] = *userIP.ID
		}
	}

	return result, joinError
}

// EnsurePublicIPs2 creates or updates PublicIPs for the NATs
func (f *FlowContext) ensurePublicIPs(ctx context.Context) (map[AzureResourceIdentifier]string, error) {
	var (
		log       = f.LogFromContext(ctx)
		joinError error
	)

	c, err := f.factory.PublicIP()
	if err != nil {
		return nil, err
	}

	var (
		toDelete    = sets.New[AzureResourceIdentifier]()
		toReconcile = map[AzureResourceIdentifier]armnetwork.PublicIPAddress{}
	)

	for _, ipFromConfig := range f.adapter.IPs() {
		if ipFromConfig.userManaged {
			continue
		}

		pip := armnetwork.PublicIPAddress{
			Location: to.Ptr(f.adapter.Region()),
			Properties: &armnetwork.PublicIPAddressPropertiesFormat{
				PublicIPAllocationMethod: to.Ptr(armnetwork.IPAllocationMethodStatic),
			},
			SKU:   &armnetwork.PublicIPAddressSKU{Name: to.Ptr(armnetwork.PublicIPAddressSKUNameStandard)},
			Zones: to.SliceOfPtrs(ipFromConfig.zones...),
		}
		toReconcile[ipFromConfig.AzureResourceIdentifier] = pip
	}

	currentIPs, err := c.List(ctx, f.adapter.ResourceGroupName())
	if err != nil {
		return nil, err
	}

	// filter only these IPs prefixed by the cluster name.
	currentIPs = Filter(currentIPs, func(address *armnetwork.PublicIPAddress) bool {
		return f.adapter.HasShootPrefix(address.Name)
	})

	for _, currentIP := range currentIPs {
		id := AzureResourceIdentifier{
			ResourceGroup: f.adapter.ResourceGroupName(),
			Name:          *currentIP.Name,
			Kind:          PublicIP,
		}
		// delete all the resources that are not in the list of target resources
		if _, ok := toReconcile[id]; !ok {
			toDelete.Insert(id)
			continue
		}

		// delete all resources whose spec cannot be updated to match target spec.
		targetIP := toReconcile[id]
		if PublicIPAddress(*currentIP).MustDelete(targetIP) {
			toDelete.Insert(id)
			continue
		}
	}

	for _, ip := range toDelete.UnsortedList() {
		log.Info("deleting IP", "IP", ip.Name)
		err := f.provider.DeletePublicIP(ctx, ip.ResourceGroup, ip.Name)
		if err != nil {
			joinError = errors.Join(joinError, err)
		}
	}
	if joinError != nil {
		return nil, joinError
	}

	result := make(map[AzureResourceIdentifier]string)
	for id, ip := range toReconcile {
		newIp, err := c.CreateOrUpdate(ctx, f.adapter.ResourceGroupName(), id.Name, ip)
		if err != nil {
			joinError = errors.Join(joinError, err)
			continue
		}

		result[id] = *newIp.ID
	}

	return result, joinError
}

func (f *FlowContext) EnsureNatGateways(ctx context.Context) error {
	ipMapping := GetObject[map[AzureResourceIdentifier]string](f.whiteboard, publicIPMapKey)
	natMapping, err := f.ensureNatGateways(ctx, ipMapping)
	f.whiteboard.SetObject(natGatewayMapKey, natMapping)
	return err
}

// EnsureNatGateways creates or updates NAT Gateways. It also deletes old NATGateways.
func (f *FlowContext) ensureNatGateways(ctx context.Context, ipMapping map[AzureResourceIdentifier]string) (map[string]string, error) {
	c, err := f.factory.NatGateway()
	if err != nil {
		return nil, err
	}

	var joinError error

	currentNats, err := c.List(ctx, f.adapter.ResourceGroupName())
	if err != nil {
		return nil, err
	}
	currentNats = Filter(currentNats, func(n *armnetwork.NatGateway) bool {
		return f.adapter.HasShootPrefix(n.Name)
	})

	zones := f.adapter.Zones()
	for _, nat := range currentNats {
		if !checkAllZonesWithFn(*nat, zones, func(zone zone, resource armnetwork.NatGateway) bool {
			return zone.natGateway != nil &&
				*resource.Name == zone.natGateway.Name &&
				reflect.DeepEqual(resource.Zones, to.SliceOfPtrs(zone.natGateway.zone))
		}) {
			joinError = errors.Join(joinError, f.provider.DeleteNatGateway(ctx, f.adapter.ResourceGroupName(), *nat.Name))
		}
	}

	result := make(map[string]string)
	for _, target := range f.adapter.Nats() {
		ngw := &armnetwork.NatGateway{
			Properties: &armnetwork.NatGatewayPropertiesFormat{
				IdleTimeoutInMinutes: target.idleTimeout,
			},
			Location: to.Ptr(f.adapter.Region()),
			SKU:      &armnetwork.NatGatewaySKU{Name: to.Ptr(armnetwork.NatGatewaySKUNameStandard)},
		}
		if target.zone != nil {
			ngw.Zones = []*string{target.zone}
		}
		for _, pip := range target.pip {
			pipId, ok := ipMapping[pip.AzureResourceIdentifier]
			if !ok {
				joinError = errors.Join(joinError, fmt.Errorf("public IP %s/%s needed for NAT Gateway %s was not found", pip.ResourceGroup, pip.Name, target.Name))
				continue
			}
			ngw.Properties.PublicIPAddresses = append(ngw.Properties.PublicIPAddresses, &armnetwork.SubResource{ID: to.Ptr(pipId)})
		}
		ngw, err = c.CreateOrUpdate(ctx, target.ResourceGroup, target.Name, *ngw)
		if err != nil {
			joinError = errors.Join(joinError, err)
			continue
		}
		result[target.Name] = *ngw.ID
	}

	return result, nil
}

func checkAllZonesWithFn[T any](t T, zones []zone, check func(zone zone, resource T) bool) bool {
	for _, n := range zones {
		if check(n, t) {
			return true
		}
	}
	return false
}

// EnsureSubnets creates or updates subnets.
func (f *FlowContext) EnsureSubnets(ctx context.Context) error {
	if err := f.ensureObjectKeys(natGatewayMapKey); err != nil {
		return fmt.Errorf("failed to ensure subnets: %v", err)
	}
	sgId := f.whiteboard.Get(securityGroupId)
	rtId := f.whiteboard.Get(routeTableId)

	natMap := GetObject[map[string]string](f.whiteboard, natGatewayMapKey)
	if sgId == nil {
		return fmt.Errorf("failed to ensure subnets: missing security group Id")
	}
	if rtId == nil {
		return fmt.Errorf("failed to ensure subnets: missing route table Id")
	}
	return f.ensureSubnets(ctx, sgId, rtId, natMap)
}

func (f *FlowContext) ensureSubnets(ctx context.Context, securityGroup, routeTable *string, natMap map[string]string) (err error) {
	vnetRgroup := f.adapter.VnetResourceGroup()
	vnetName := f.adapter.VnetName()

	c, err := f.factory.Subnet()
	if err != nil {
		return err
	}

	currentSubnets, err := c.List(ctx, vnetRgroup, vnetName)
	if err != nil {
		return err
	}

	filteredSubnets := Filter(currentSubnets, func(s *armnetwork.Subnet) bool {
		return f.adapter.HasShootPrefix(s.Name)
	})

	zones := f.adapter.Zones()

	var joinErr error
	for _, subnet := range filteredSubnets {
		if !checkAllZonesWithFn(*subnet.Name, zones, func(zone zone, name string) bool {
			return name == zone.subnet.Name
		}) {
			joinErr = errors.Join(joinErr, c.Delete(ctx, vnetRgroup, vnetName, *subnet.Name))
		}
	}

	subnetsMap := ToMap(filteredSubnets, func(subnet *armnetwork.Subnet) string {
		if subnet == nil || subnet.Name == nil {
			return ""
		}
		return *subnet.Name
	})

	for _, zone := range zones {
		subnet := &armnetwork.Subnet{}
		if s, ok := subnetsMap[zone.subnet.Name]; ok {
			subnet = s
		}

		if subnet.Properties == nil {
			subnet.Properties = &armnetwork.SubnetPropertiesFormat{}
		}

		subnet.Properties.ServiceEndpoints = make([]*armnetwork.ServiceEndpointPropertiesFormat, 0)
		for _, endpoint := range zone.subnet.serviceEndpoint {
			subnet.Properties.ServiceEndpoints = append(subnet.Properties.ServiceEndpoints, &armnetwork.ServiceEndpointPropertiesFormat{
				Service: to.Ptr(endpoint),
			})
		}
		subnet.Properties.AddressPrefixes = []*string{to.Ptr(zone.subnet.cidr)}
		subnet.Properties.NetworkSecurityGroup = &armnetwork.SecurityGroup{
			ID: securityGroup,
		}
		subnet.Properties.RouteTable = &armnetwork.RouteTable{
			ID: routeTable,
		}
		if zone.natGateway != nil {
			subnet.Properties.NatGateway = &armnetwork.SubResource{ID: to.Ptr(natMap[zone.natGateway.Name])}
		}

		_, err = c.CreateOrUpdate(ctx, vnetRgroup, vnetName, zone.subnet.Name, *subnet)
		if err != nil {
			joinErr = errors.Join(joinErr, err)
			continue
		}
	}

	return joinErr
}

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
