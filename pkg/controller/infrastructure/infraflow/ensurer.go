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

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v5"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v4"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/pointer"

	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/v1alpha1"
	"github.com/gardener/gardener-extension-provider-azure/pkg/internal/infrastructure"
)

// Key names for the whiteboard object to pass results between the reconcilation tasks
const (
	routeTableIdKey    = "route_table_id"
	securityGroupIdKey = "security_group_id"
	// natGatewayMapKey   = "nategateway_map"
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

	vnet = vnetCfg.ToProvider(vnet)
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
	// ipMap, err := f.ensurePublicIPs(ctx)
	// userIpMap, usrErr := f.ensureUserPublicIps(ctx)
	err := f.ensurePublicIPs(ctx)
	usrErr := f.ensureUserPublicIps(ctx)

	// ipMap = Join(ipMap, userIpMap)
	// f.whiteboard.SetObject(publicIPMapKey, ipMap)
	return errors.Join(err, usrErr)
}

// func (f *FlowContext) ensureUserPublicIps(ctx context.Context) (map[AzureResourceMetadata]string, error) {
func (f *FlowContext) ensureUserPublicIps(ctx context.Context) error {
	var (
		joinError error
		// result    map[AzureResourceMetadata]string
	)

	c, err := f.factory.PublicIP()
	if err != nil {
		return err
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
			// } else {
			// 	result[ipFromConfig.AzureResourceMetadata] = *userIP.ID
		}
	}

	return joinError
}

// EnsurePublicIPs2 creates or updates PublicIPs for the NATs
// func (f *FlowContext) ensurePublicIPs(ctx context.Context) (map[AzureResourceMetadata]string, error) {
func (f *FlowContext) ensurePublicIPs(ctx context.Context) error {
	var (
		log         = f.LogFromContext(ctx)
		toDelete    = sets.New[string]()
		toReconcile = map[string]*armnetwork.PublicIPAddress{}
		joinError   error
	)

	c, err := f.factory.PublicIP()
	if err != nil {
		// return nil, err
		return err
	}

	currentIPs, err := c.List(ctx, f.adapter.ResourceGroup())
	if err != nil {
		// return nil, err
		return err
	}

	// filter only these IpConfigs prefixed by the cluster name.
	currentIPs = Filter(currentIPs, func(address *armnetwork.PublicIPAddress) bool {
		return f.adapter.HasShootPrefix(address.Name)
	})
	mappedIps := ToMap(currentIPs, func(t *armnetwork.PublicIPAddress) string {
		return *t.Name
	})

	managedIPConfigs := f.adapter.ManagedIpConfigs()
	for name, pipCfg := range managedIPConfigs {
		toReconcile[name] = pipCfg.ToProvider(mappedIps[name])
	}

	for name, current := range mappedIps {
		// delete all the resources that are not in the list of target resources
		pipCfg, ok := managedIPConfigs[name]
		if !ok {
			log.Info("will delete public IP because it is not needed", "Resource Group", f.adapter.ResourceGroup(), "Name", name)
			toDelete.Insert(name)
			continue
		}

		// delete all resources whose spec cannot be updated to match target spec.
		if ok, offender, v := ForceNewIp(current, toReconcile[pipCfg.Name]); !ok {
			log.Info("will delete public IP because it can't be reconciled", "Resource Group", f.adapter.ResourceGroup(), "Name", name, "Offender", offender, "Value", v)
			toDelete.Insert(name)
			continue
		}
	}

	for _, ipName := range toDelete.UnsortedList() {
		err := f.provider.DeletePublicIP(ctx, f.adapter.ResourceGroup(), ipName)
		if err != nil {
			joinError = errors.Join(joinError, err)
		}
	}
	if joinError != nil {
		return joinError
	}

	for ipName, ip := range toReconcile {
		_, err := c.CreateOrUpdate(ctx, f.adapter.ResourceGroup(), ipName, *ip)
		if err != nil {
			joinError = errors.Join(joinError, err)
			continue
		}
	}

	return joinError
}

func (f *FlowContext) EnsureNatGateways(ctx context.Context) error {
	err := f.ensureNatGateways(ctx)
	return err
}

// EnsureNatGateways creates or updates NAT Gateways. It also deletes old NATGateways.
func (f *FlowContext) ensureNatGateways(ctx context.Context) error {
	// func (f *FlowContext) ensureNatGateways(ctx context.Context, ipMapping map[AzureResourceMetadata]string) error {
	var (
		log         = f.LogFromContext(ctx)
		joinError   error
		toDelete    = sets.New[string]()
		toReconcile = map[string]*armnetwork.NatGateway{}
	)

	c, err := f.factory.NatGateway()
	if err != nil {
		return err
	}

	nats, err := c.List(ctx, f.adapter.ResourceGroup())
	if err != nil {
		return err
	}
	nats = Filter(nats, func(n *armnetwork.NatGateway) bool {
		return f.adapter.HasShootPrefix(n.Name)
	})
	mappedNats := ToMap(nats, func(t *armnetwork.NatGateway) string {
		return *t.Name
	})

	natsCfg := f.adapter.NatGatewayConfigs()
	for name, cfg := range natsCfg {
		target := cfg.ToProvider(mappedNats[name])
		for _, ip := range cfg.PublicIPList {
			target.Properties.PublicIPAddresses = append(target.Properties.PublicIPAddresses, &armnetwork.SubResource{ID: to.Ptr(GetIdFromTemplate(PublicIPTemplate, f.auth.SubscriptionID, ip.ResourceGroup, ip.Name))})
		}
		toReconcile[name] = target
	}

	for name, current := range mappedNats {
		target, ok := toReconcile[name]
		if !ok {
			log.Info("will delete NAT Gateway because it is not needed", "Resource Group", f.adapter.ResourceGroup(), "Name", *current.Name)
			toDelete.Insert(*current.Name)
			continue
		}
		if ok, offender, v := ForceNewNat(current, target); ok {
			log.Info("will delete NAT Gateway because it cannot be reconciled", "Resource Group", f.adapter.ResourceGroup(), "Name", *current.Name, "Offender", offender, "Value", v)
			toDelete.Insert(*current.Name)
			continue
		}
	}

	for _, natName := range toDelete.UnsortedList() {
		err := f.provider.DeleteNatGateway(ctx, f.adapter.ResourceGroup(), natName)
		if err != nil {
			joinError = errors.Join(joinError, err)
		}
	}
	if joinError != nil {
		return joinError
	}

	for name, nat := range toReconcile {
		_, err := c.CreateOrUpdate(ctx, f.adapter.ResourceGroup(), name, *nat)
		if err != nil {
			joinError = errors.Join(joinError, err)
			continue
		}
	}
	return joinError
}

// EnsureSubnets creates or updates subnets.
func (f *FlowContext) EnsureSubnets(ctx context.Context) error {
	return f.ensureSubnets(ctx)
}

func (f *FlowContext) ensureSubnets(ctx context.Context) (err error) {
	var (
		log         = f.LogFromContext(ctx)
		vnetRgroup  = f.adapter.VirtualNetworkConfig().ResourceGroup
		vnetName    = f.adapter.VirtualNetworkConfig().Name
		toDelete    = sets.New[string]()
		toReconcile = map[string]*armnetwork.Subnet{}
		joinErr     error
	)

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
	mappedSubnets := ToMap(filteredSubnets, func(s *armnetwork.Subnet) string {
		return *s.Name
	})

	zones := f.adapter.Zones()
	for _, z := range zones {
		actual := z.Subnet.ToProvider(mappedSubnets[z.Subnet.Name])
		rtCfg := f.adapter.RouteTableConfig()
		sgCfg := f.adapter.RouteTableConfig()
		actual.Properties.RouteTable = &armnetwork.RouteTable{ID: to.Ptr(GetIdFromTemplate(RouteTableTemplate, f.auth.SubscriptionID, rtCfg.ResourceGroup, rtCfg.Name))}
		actual.Properties.NetworkSecurityGroup = &armnetwork.SecurityGroup{ID: to.Ptr(GetIdFromTemplate(SecurityGroupTemplate, f.auth.SubscriptionID, sgCfg.ResourceGroup, sgCfg.Name))}
		if z.NatGateway != nil {
			actual.Properties.NatGateway = &armnetwork.SubResource{ID: to.Ptr(NatGatewayId(f.auth.SubscriptionID, z.NatGateway.ResourceGroup, z.NatGateway.Name))}
		}
		toReconcile[z.Subnet.Name] = actual
	}

	for name, current := range mappedSubnets {
		target, ok := toReconcile[name]
		if !ok {
			log.Info("will delete Subnet because it is not needed", "Resource Group", vnetRgroup, "Name", *current.Name)
			toDelete.Insert(name)
			continue
		}
		if ok, offender, v := ForceNewSubnet(current, target); ok {
			log.Info("will delete Subnet because it cannot be reconciled", "Resource Group", vnetRgroup, "Name", *current.Name, "Offender", offender, "Value", v)
			toDelete.Insert(name)
			continue
		}
	}

	for _, name := range toDelete.UnsortedList() {
		err := c.Delete(ctx, vnetRgroup, vnetName, name)
		if err != nil {
			joinErr = errors.Join(joinErr, err)
		}
	}
	if joinErr != nil {
		return joinErr
	}

	for name, subnet := range toReconcile {
		_, err = c.CreateOrUpdate(ctx, vnetRgroup, vnetName, name, *subnet)
		if err != nil {
			joinErr = errors.Join(joinErr, err)
			continue
		}
	}

	return joinErr
}

// GetInfrastructureStatus returns the infrastructure status
func (f *FlowContext) GetInfrastructureStatus(ctx context.Context) (*v1alpha1.InfrastructureStatus, error) {
	status := &v1alpha1.InfrastructureStatus{
		TypeMeta: infrastructure.StatusTypeMeta,
		Networks: v1alpha1.NetworkStatus{
			VNet: v1alpha1.VNetStatus{
				Name:          f.adapter.VirtualNetworkConfig().ResourceGroup,
				ResourceGroup: to.Ptr(f.adapter.VirtualNetworkConfig().ResourceGroup),
			},
			Layout: v1alpha1.NetworkLayoutSingleSubnet,
		},
		ResourceGroup: v1alpha1.ResourceGroup{
			Name: f.adapter.ResourceGroup(),
		},
		RouteTables: []v1alpha1.RouteTable{
			{
				Purpose: v1alpha1.PurposeNodes,
				Name:    f.adapter.RouteTableConfig().Name,
			},
		},
		SecurityGroups: []v1alpha1.SecurityGroup{
			{
				Purpose: v1alpha1.PurposeNodes,
				Name:    f.adapter.SecurityGroupConfig().Name,
			},
		},
		Zoned: f.cfg.Zoned,
	}

	if len(f.cfg.Networks.Zones) > 0 {
		status.Networks.Layout = v1alpha1.NetworkLayoutMultipleSubnet
	}

	zones := f.adapter.Zones()
	for _, z := range zones {
		status.Networks.Subnets = append(status.Networks.Subnets, v1alpha1.Subnet{
			Name:     z.Subnet.Name,
			Purpose:  v1alpha1.PurposeNodes,
			Zone:     z.Subnet.zone,
			Migrated: z.Migrated,
		})
	}

	if cfg := f.adapter.AvailabilitySetConfig(); cfg != nil {
		status.AvailabilitySets = []v1alpha1.AvailabilitySet{
			{
				Purpose:            v1alpha1.PurposeNodes,
				ID:                 GetIdFromTemplate(AvailabilitySetIDTemplate, f.auth.SubscriptionID, cfg.ResourceGroup, cfg.Name),
				Name:               cfg.Name,
				CountFaultDomains:  cfg.CountFaultDomains,
				CountUpdateDomains: cfg.CountUpdateDomains,
			},
		}
	}

	err := f.enrichStatusWithIdentity(ctx, status)
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
	return c.Delete(ctx, f.adapter.ResourceGroup())
}

// deleteSubnetsInForeignGroup deletes all managed subnets in a foreign resource group
func (f *FlowContext) deleteSubnetsInForeignGroup(ctx context.Context) error {
	vnetCfg := f.adapter.VirtualNetworkConfig()
	if vnetCfg.Managed {
		return nil
	}

	vnetRgroup := vnetCfg.ResourceGroup
	vnetName := vnetCfg.Name

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

	var joinErr error
	for _, s := range filteredSubnets {
		err := c.Delete(ctx, vnetRgroup, vnetName, *s.Name)
		if err != nil {
			joinErr = errors.Join(joinErr, err)
			continue
		}
	}
	return joinErr
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
