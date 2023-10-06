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
	routeTableIdKey    = "route_table_id"
	securityGroupIdKey = "security_group_id"
	natGatewayMapKey   = "nategateway_map"
	publicIPMapKey     = "public-ips"
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

	_, err = rgClient.CreateOrUpdate(ctx, f.adapter.ResourceGroup(), rg)
	return err
}

func (f *FlowContext) EnsureVirtualNetwork(ctx context.Context) error {
	if f.adapter.VirtualNetworkConfig().Managed {
		return f.ensureManagedVirtualNetwork(ctx)
	}

	return f.ensureUserVirtualNetwork(ctx)
}

// EnsureVirtualNetwork creates or updates a Vnet
func (f *FlowContext) ensureManagedVirtualNetwork(ctx context.Context) error {
	vnetCfg := f.adapter.VirtualNetworkConfig()

	c, err := f.factory.Vnet()
	if err != nil {
		return err
	}

	vnet, err := c.Get(ctx, vnetCfg.ResourceGroup, vnetCfg.Name)
	if err != nil {
		return err
	}

	if vnet != nil {
		if pointer.StringDeref(vnet.Location, "") != f.adapter.Region() {
			return NewTerminalSpecMismatch(vnetCfg.AzureResourceMetadata, "Location", f.adapter.Region())
		}
	} else {
		vnet = &armnetwork.VirtualNetwork{}
	}

	vnet = f.applyTargetVnet(vnet)
	_, err = c.CreateOrUpdate(ctx, vnetCfg.ResourceGroup, vnetCfg.Name, *vnet)
	return err
}

func (f *FlowContext) ensureUserVirtualNetwork(ctx context.Context) error {
	vnetCfg := f.adapter.VirtualNetworkConfig()

	c, err := f.factory.Vnet()
	if err != nil {
		return err
	}

	vnet, err := c.Get(ctx, vnetCfg.ResourceGroup, vnetCfg.Name)
	if err != nil {
		return err
	}

	if vnet == nil {
		return NewTerminalConditionError(vnetCfg.AzureResourceMetadata, fmt.Errorf("user vnet not found"))
	}

	return nil
}

// EnsureAvailabilitySet creates or updates an AvailabilitySet
func (f *FlowContext) EnsureAvailabilitySet(ctx context.Context) error {
	log := f.LogFromContext(ctx)
	avsetCfg := f.adapter.AvailabilitySetConfig()
	if avsetCfg == nil {
		log.Info("skipping ensuring availability set")
		return nil
	}

	asClient, err := f.factory.AvailabilitySet()
	if err != nil {
		return err
	}
	parameters := armcompute.AvailabilitySet{
		Location: to.Ptr(f.adapter.Region()),
		// the DomainCounts are computed from the current InfrastructureStatus. They cannot be updated after shoot creation.
		Properties: &armcompute.AvailabilitySetProperties{
			PlatformFaultDomainCount:  avsetCfg.CountFaultDomains,
			PlatformUpdateDomainCount: avsetCfg.CountUpdateDomains,
		},
		SKU: &armcompute.SKU{Name: to.Ptr(string(armcompute.AvailabilitySetSKUTypesAligned))}, // equal to managed = True in tf
	}
	_, err = asClient.CreateOrUpdate(ctx, f.adapter.ResourceGroup(), avsetCfg.Name, parameters)
	return err
}

// EnsureRouteTable creates or updates the route table
func (f *FlowContext) EnsureRouteTable(ctx context.Context) error {
	routeTable, err := f.ensureRouteTable(ctx)
	f.whiteboard.Set(routeTableIdKey, *routeTable.ID)
	return err
}

// EnsureRouteTables creates or updates a RouteTable
func (f *FlowContext) ensureRouteTable(ctx context.Context) (*armnetwork.RouteTable, error) {
	rtCfg := f.adapter.RouteTableConfig()

	c, err := f.factory.RouteTables()
	if err != nil {
		return nil, err
	}

	rt, err := c.Get(ctx, rtCfg.ResourceGroup, rtCfg.Name)
	if err != nil {
		return nil, err
	}

	if rt != nil {
		if pointer.StringDeref(rt.Location, "") != f.adapter.Region() {
			return nil, NewTerminalSpecMismatch(rtCfg.AzureResourceMetadata, "Location", f.adapter.Region())
		}
	}

	// create the RT
	if rt == nil {
		parameters := armnetwork.RouteTable{
			Location:   to.Ptr(f.adapter.Region()),
			Properties: &armnetwork.RouteTablePropertiesFormat{},
		}

		return c.CreateOrUpdate(ctx, rtCfg.ResourceGroup, rtCfg.Name, parameters)
	}

	return rt, nil
}

// EnsureSecurityGroup creates or updates a SecurityGroup
func (f *FlowContext) EnsureSecurityGroup(ctx context.Context) error {
	sg, err := f.ensureSecurityGroup(ctx)
	if err != nil {
		return err
	}

	f.whiteboard.Set(securityGroupIdKey, *sg.ID)
	return nil
}

func (f *FlowContext) ensureSecurityGroup(ctx context.Context) (*armnetwork.SecurityGroup, error) {
	sgCfg := f.adapter.SecurityGroupConfig()

	c, err := f.factory.NetworkSecurityGroup()
	if err != nil {
		return nil, err
	}

	nsg, err := c.Get(ctx, sgCfg.ResourceGroup, sgCfg.Name)
	if err != nil {
		return nil, err
	}

	if nsg != nil {
		// if the location doesn't match, attempt to delete the NSG.
		if pointer.StringDeref(nsg.Location, "") != f.adapter.Region() {
			return nil, NewTerminalSpecMismatch(sgCfg.AzureResourceMetadata, "Location", f.adapter.Region())
		}
	}

	// create the NSG if it not there
	if nsg == nil {
		parameters := armnetwork.SecurityGroup{
			Location:   to.Ptr(f.adapter.Region()),
			Properties: &armnetwork.SecurityGroupPropertiesFormat{},
		}
		return c.CreateOrUpdate(ctx, sgCfg.ResourceGroup, sgCfg.Name, parameters)
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

func (f *FlowContext) ensureUserPublicIps(ctx context.Context) (map[AzureResourceMetadata]string, error) {
	var (
		joinError error
		result    map[AzureResourceMetadata]string
	)

	c, err := f.factory.PublicIP()
	if err != nil {
		return nil, err
	}

	for _, ipFromConfig := range f.adapter.IpConfigs() {
		if !ipFromConfig.Managed {
			continue
		}

		userIP, err := c.Get(ctx, ipFromConfig.ResourceGroup, ipFromConfig.Name, nil)
		if err != nil {
			joinError = errors.Join(joinError, err)
		} else if userIP == nil {
			joinError = errors.Join(joinError, fmt.Errorf(fmt.Sprintf("failed to locate user IP: %s, %s", "", "")))
		} else {
			result[ipFromConfig.AzureResourceMetadata] = *userIP.ID
		}
	}

	return result, joinError
}

// EnsurePublicIPs2 creates or updates PublicIPs for the NATs
func (f *FlowContext) ensurePublicIPs(ctx context.Context) (map[AzureResourceMetadata]string, error) {
	var (
		log       = f.LogFromContext(ctx)
		joinError error
	)

	c, err := f.factory.PublicIP()
	if err != nil {
		return nil, err
	}

	var (
		toDelete    = sets.New[AzureResourceMetadata]()
		toReconcile = map[AzureResourceMetadata]armnetwork.PublicIPAddress{}
	)

	for _, ipFromConfig := range f.adapter.IpConfigs() {
		if ipFromConfig.Managed {
			continue
		}

		pip := armnetwork.PublicIPAddress{
			Location: to.Ptr(f.adapter.Region()),
			Properties: &armnetwork.PublicIPAddressPropertiesFormat{
				PublicIPAllocationMethod: to.Ptr(armnetwork.IPAllocationMethodStatic),
			},
			SKU:   &armnetwork.PublicIPAddressSKU{Name: to.Ptr(armnetwork.PublicIPAddressSKUNameStandard)},
			Zones: to.SliceOfPtrs(ipFromConfig.Zones...),
		}
		toReconcile[ipFromConfig.AzureResourceMetadata] = pip
	}

	currentIPs, err := c.List(ctx, f.adapter.ResourceGroup())
	if err != nil {
		return nil, err
	}

	// filter only these IpConfigs prefixed by the cluster name.
	currentIPs = Filter(currentIPs, func(address *armnetwork.PublicIPAddress) bool {
		return f.adapter.HasShootPrefix(address.Name)
	})

	for _, currentIP := range currentIPs {
		id := AzureResourceMetadata{
			ResourceGroup: f.adapter.ResourceGroup(),
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

	result := make(map[AzureResourceMetadata]string)
	for id, ip := range toReconcile {
		newIp, err := c.CreateOrUpdate(ctx, f.adapter.ResourceGroup(), id.Name, ip)
		if err != nil {
			joinError = errors.Join(joinError, err)
			continue
		}

		result[id] = *newIp.ID
	}

	return result, joinError
}

func (f *FlowContext) EnsureNatGateways(ctx context.Context) error {
	ipMapping := GetObject[map[AzureResourceMetadata]string](f.whiteboard, publicIPMapKey)
	natMapping, err := f.ensureNatGateways(ctx, ipMapping)
	f.whiteboard.SetObject(natGatewayMapKey, natMapping)
	return err
}

// EnsureNatGateways creates or updates NAT Gateways. It also deletes old NATGateways.
func (f *FlowContext) ensureNatGateways(ctx context.Context, ipMapping map[AzureResourceMetadata]string) (map[string]string, error) {
	c, err := f.factory.NatGateway()
	if err != nil {
		return nil, err
	}

	var joinError error

	currentNats, err := c.List(ctx, f.adapter.ResourceGroup())
	if err != nil {
		return nil, err
	}
	currentNats = Filter(currentNats, func(n *armnetwork.NatGateway) bool {
		return f.adapter.HasShootPrefix(n.Name)
	})

	zones := f.adapter.Zones()
	for _, nat := range currentNats {
		if !checkAllZonesWithFn(*nat, zones, func(zone ZoneConfig, resource armnetwork.NatGateway) bool {
			return zone.NatGateway != nil &&
				*resource.Name == zone.NatGateway.Name &&
				reflect.DeepEqual(resource.Zones, to.SliceOfPtrs(zone.NatGateway.Zone))
		}) {
			joinError = errors.Join(joinError, f.provider.DeleteNatGateway(ctx, f.adapter.ResourceGroup(), *nat.Name))
		}
	}

	result := make(map[string]string)
	for _, target := range f.adapter.NatGatewayConfigs() {
		ngw := &armnetwork.NatGateway{
			Properties: &armnetwork.NatGatewayPropertiesFormat{
				IdleTimeoutInMinutes: target.IdleTimeout,
			},
			Location: to.Ptr(f.adapter.Region()),
			SKU:      &armnetwork.NatGatewaySKU{Name: to.Ptr(armnetwork.NatGatewaySKUNameStandard)},
		}
		if target.Zone != nil {
			ngw.Zones = []*string{target.Zone}
		}
		for _, pip := range target.PublicIPList {
			pipId, ok := ipMapping[pip.AzureResourceMetadata]
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

func checkAllZonesWithFn[T any](t T, zones []ZoneConfig, check func(zone ZoneConfig, resource T) bool) bool {
	for _, n := range zones {
		if check(n, t) {
			return true
		}
	}
	return false
}

// EnsureSubnets creates or updates subnets.
func (f *FlowContext) EnsureSubnets(ctx context.Context) error {
	if err := EnsureObjectKeys(f.whiteboard, natGatewayMapKey); err != nil {
		return fmt.Errorf("failed to ensure subnets: %v", err)
	}
	sgId := f.whiteboard.Get(securityGroupIdKey)
	rtId := f.whiteboard.Get(routeTableIdKey)

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
		if !checkAllZonesWithFn(*subnet.Name, zones, func(zone ZoneConfig, name string) bool {
			return name == zone.Subnet.Name
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
		if s, ok := subnetsMap[zone.Subnet.Name]; ok {
			subnet = s
		}

		if subnet.Properties == nil {
			subnet.Properties = &armnetwork.SubnetPropertiesFormat{}
		}

		subnet.Properties.ServiceEndpoints = make([]*armnetwork.ServiceEndpointPropertiesFormat, 0)
		for _, endpoint := range zone.Subnet.serviceEndpoint {
			subnet.Properties.ServiceEndpoints = append(subnet.Properties.ServiceEndpoints, &armnetwork.ServiceEndpointPropertiesFormat{
				Service: to.Ptr(endpoint),
			})
		}
		subnet.Properties.AddressPrefixes = []*string{to.Ptr(zone.Subnet.cidr)}
		subnet.Properties.NetworkSecurityGroup = &armnetwork.SecurityGroup{
			ID: securityGroup,
		}
		subnet.Properties.RouteTable = &armnetwork.RouteTable{
			ID: routeTable,
		}
		if zone.NatGateway != nil {
			subnet.Properties.NatGateway = &armnetwork.SubResource{ID: to.Ptr(natMap[zone.NatGateway.Name])}
		}

		_, err = c.CreateOrUpdate(ctx, vnetRgroup, vnetName, zone.Subnet.Name, *subnet)
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
	vnetCfg := f.adapter.VirtualNetworkConfig()
	if v.Properties == nil {
		v.Properties = &armnetwork.VirtualNetworkPropertiesFormat{}
	}

	v.Location = to.Ptr(f.adapter.Region())
	v.Properties.AddressSpace = &armnetwork.AddressSpace{
		AddressPrefixes: []*string{vnetCfg.CIDR},
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
			return fmt.Errorf("failed to delete foreign SubnetConfig: %w", err)
		}
	}
	return nil
}

// EnrichResponseWithUserManagedIPs adds the IDs of user managed IpConfigs to the input map of associated IpConfigs of the NATs
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
