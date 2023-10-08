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
	"fmt"
	"strconv"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v4"
	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"

	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/helper"
	consts "github.com/gardener/gardener-extension-provider-azure/pkg/azure"
	"github.com/gardener/gardener-extension-provider-azure/pkg/internal/infrastructure"
)

// InfrastructureAdapter contains information about the infrastructure resources that are either static, or otherwise
// inferable based on the shoot configuration. It acts as an intermediate step to make the configuration easier to process
// for the ensurer step.
type InfrastructureAdapter struct {
	infra   *extensionsv1alpha1.Infrastructure
	config  *azure.InfrastructureConfig
	status  *azure.InfrastructureStatus
	profile *azure.CloudProfileConfig
	cluster *extensionscontroller.Cluster

	// cached configuration
	vnetConfig  VirtualNetworkConfig
	avSetConfig *AvailabilitySetConfig
	zoneConfigs []ZoneConfig
}

// NewInfrastructureAdapter returns a new instance of the InfrastructureAdapter.
func NewInfrastructureAdapter(
	infra *extensionsv1alpha1.Infrastructure,
	config *azure.InfrastructureConfig,
	status *azure.InfrastructureStatus,
	profile *azure.CloudProfileConfig,
	cluster *extensionscontroller.Cluster,
) (*InfrastructureAdapter, error) {
	ia := &InfrastructureAdapter{
		infra:   infra,
		config:  config,
		status:  status,
		profile: profile,
		cluster: cluster,
	}
	ia.vnetConfig = ia.virtualNetworkConfig()
	avset, err := ia.availabilitySetConfig()
	if err != nil {
		return nil, err
	}
	ia.avSetConfig = avset

	ia.zoneConfigs = ia.zonesConfig()
	return ia, nil
}

// TechnicalName the cluster's "base" name. Used as a name or as a prefix by other resources.
func (ia *InfrastructureAdapter) TechnicalName() string {
	return ia.infra.Namespace
}

// ResourceGroup the name of the resource group.
func (ia *InfrastructureAdapter) ResourceGroup() string {
	return ia.TechnicalName()
}

// VirtualNetworkConfig contains configuration for the virtual network
type VirtualNetworkConfig struct {
	AzureResourceMetadata
	// Managed is true if the vnet is managed by gardener.
	Managed  bool
	Location string
	// Cidr is the vnet's CIDR.
	CIDR     *string
	DDoSPlan *string
}

// Region is the region of the shoot.
func (ia *InfrastructureAdapter) Region() string {
	return ia.infra.Spec.Region
}

// VirtualNetworkConfig returns the virtual network configuration.
func (ia *InfrastructureAdapter) VirtualNetworkConfig() VirtualNetworkConfig {
	return ia.vnetConfig
}

func (ia *InfrastructureAdapter) virtualNetworkConfig() VirtualNetworkConfig {
	name := ia.TechnicalName()
	rg := ia.ResourceGroup()
	managed := ia.isGardenerManagedVirtualNetwork()
	if !managed {
		name = *ia.config.Networks.VNet.Name
		rg = *ia.config.Networks.VNet.ResourceGroup
	}
	vnc := VirtualNetworkConfig{
		AzureResourceMetadata: AzureResourceMetadata{
			Name:          name,
			ResourceGroup: rg,
			Kind:          VirtualNetwork,
		},
		Managed:  managed,
		Location: ia.Region(),
		DDoSPlan: ia.config.Networks.VNet.DDosProtectionPlanID,
	}

	if cidr := ia.config.Networks.VNet.CIDR; cidr != nil {
		// copy string
		vnc.CIDR = to.Ptr(*cidr)
	} else {
		vnc.CIDR = to.Ptr(*ia.config.Networks.Workers)
	}

	return vnc
}

// isGardenerManagedVirtualNetwork returns true if gardener manages the shoot's virtual network.
func (ia *InfrastructureAdapter) isGardenerManagedVirtualNetwork() bool {
	return ia.config.Networks.VNet.ResourceGroup == nil
}

// AvailabilitySetConfig contains the configuration for the shoot's availability set.
type AvailabilitySetConfig struct {
	AzureResourceMetadata
	// countFaultDomains is the fault domain count for the AV set.
	CountFaultDomains *int32
	// countFaultDomains is the update domain count for the AV set.
	CountUpdateDomains *int32
}

// AvailabilitySetRequired returns true if gardener should create an availability set for the shoot.
func (ia *InfrastructureAdapter) availabilitySetRequired() (bool, error) {
	return infrastructure.IsPrimaryAvailabilitySetRequired(ia.infra, ia.config, ia.cluster)
}

func (ia *InfrastructureAdapter) AvailabilitySetConfig() *AvailabilitySetConfig {
	return ia.avSetConfig
}

// AvailabilitySetConfig returns the availability set's configuration.
func (ia *InfrastructureAdapter) availabilitySetConfig() (*AvailabilitySetConfig, error) {
	if ok, err := ia.availabilitySetRequired(); err != nil {
		return nil, err
	} else if !ok {
		return nil, nil
	}

	asc := &AvailabilitySetConfig{
		AzureResourceMetadata: AzureResourceMetadata{
			ResourceGroup: ia.ResourceGroup(),
			Name:          fmt.Sprintf("%s-avset-workers", ia.TechnicalName()),
			Kind:          AvailabilitySet,
		},
	}

	if ia.status != nil {
		nodesAVSet, err := helper.FindAvailabilitySetByPurpose(ia.status.AvailabilitySets, azure.PurposeNodes)
		if err != nil {
			return nil, fmt.Errorf("error obtaining update and fault domain counts from infrastructure status: %v", err)
		}
		asc.CountFaultDomains = nodesAVSet.CountFaultDomains
		asc.CountUpdateDomains = nodesAVSet.CountUpdateDomains
	}

	if asc.CountFaultDomains == nil {
		count, err := helper.FindDomainCountByRegion(ia.profile.CountFaultDomains, ia.Region())
		if err != nil {
			return nil, err
		}
		asc.CountFaultDomains = to.Ptr(count)
	}
	if asc.CountUpdateDomains == nil {
		count, err := helper.FindDomainCountByRegion(ia.profile.CountUpdateDomains, ia.Region())
		if err != nil {
			return nil, err
		}
		asc.CountUpdateDomains = to.Ptr(count)
	}

	return asc, nil
}

type RouteTableConfig struct {
	AzureResourceMetadata
}

// RouteTableConfig returns the name of the shoot's route table.
func (ia *InfrastructureAdapter) RouteTableConfig() RouteTableConfig {
	return RouteTableConfig{
		AzureResourceMetadata{
			ResourceGroup: ia.ResourceGroup(),
			Name:          "worker_route_table",
			Kind:          RouteTable,
		},
	}
}

type SecurityGroupConfig struct {
	AzureResourceMetadata
}

// SecurityGroupConfig returns the name of the shoot's security group.
func (ia *InfrastructureAdapter) SecurityGroupConfig() SecurityGroupConfig {
	return SecurityGroupConfig{
		AzureResourceMetadata{
			ResourceGroup: ia.ResourceGroup(),
			Name:          fmt.Sprintf("%s-workers", ia.TechnicalName()),
			Kind:          SecurityGroup,
		},
	}
}

// PublicIPConfig contains configuration for a puplic IP resource.
type PublicIPConfig struct {
	AzureResourceMetadata
	Zones    []string
	Location string
	Managed  bool
}

// NatGatewayConfig contains configuration for a NAT Gateway.
type NatGatewayConfig struct {
	AzureResourceMetadata
	Location     string
	Zone         *string
	IdleTimeout  *int32
	PublicIPList []PublicIPConfig
}

type SubnetConfig struct {
	AzureResourceMetadata
	cidr            string
	serviceEndpoint []string
	zone            *string
}

type ZoneConfig struct {
	Subnet     SubnetConfig
	NatGateway *NatGatewayConfig
	Migrated   bool
}

func (ia *InfrastructureAdapter) natGatewayName() string {
	return fmt.Sprintf("%s-nat-gateway", ia.TechnicalName())
}

func (ia *InfrastructureAdapter) natGatewayNameForZone(zone int32, migrated bool) string {
	if migrated {
		return ia.natGatewayName()
	}

	return fmt.Sprintf("%s-z%d", ia.natGatewayName(), zone)
}

func (ia *InfrastructureAdapter) subnetName(zone *int32) string {
	n := fmt.Sprintf("%s-nodes", ia.TechnicalName())
	if zone != nil {
		n = fmt.Sprintf("%s-z%d", n, *zone)
	}
	return n
}

func (ia *InfrastructureAdapter) publicIPName(natName string) string {
	return fmt.Sprintf("%s-ip", natName)
}

func (ia *InfrastructureAdapter) Zones() []ZoneConfig {
	return ia.zoneConfigs
}

func (ia *InfrastructureAdapter) zonesConfig() []ZoneConfig {
	if len(ia.config.Networks.Zones) == 0 {
		return ia.defaultZone()
	}

	var zones []ZoneConfig
	migratedZone, ok := ia.infra.Annotations[consts.NetworkLayoutZoneMigrationAnnotation]
	for _, configZone := range ia.config.Networks.Zones {
		zoneString := helper.InfrastructureZoneToString(configZone.Name)
		isMigratedZone := ok && migratedZone == zoneString
		z := ZoneConfig{
			Subnet: SubnetConfig{
				AzureResourceMetadata: AzureResourceMetadata{
					ResourceGroup: ia.vnetConfig.ResourceGroup,
					Name:          ia.subnetName(&configZone.Name),
					Parent:        ia.vnetConfig.Name,
					Kind:          Subnet,
				},
				cidr:            configZone.CIDR,
				serviceEndpoint: configZone.ServiceEndpoints,
				zone:            &zoneString,
			},
			Migrated: isMigratedZone,
		}

		if configZone.NatGateway != nil && configZone.NatGateway.Enabled {
			ngw := &NatGatewayConfig{
				AzureResourceMetadata: AzureResourceMetadata{
					ResourceGroup: ia.ResourceGroup(),
					Name:          ia.natGatewayNameForZone(configZone.Name, isMigratedZone),
					Kind:          NatGateway,
				},
				IdleTimeout: configZone.NatGateway.IdleConnectionTimeoutMinutes,
				Location:    ia.Region(),
				Zone:        to.Ptr(zoneString),
			}
			z.NatGateway = ngw

			if len(configZone.NatGateway.IPAddresses) > 0 {
				for _, ipRef := range configZone.NatGateway.IPAddresses {
					ip := PublicIPConfig{
						AzureResourceMetadata: AzureResourceMetadata{
							ResourceGroup: ipRef.ResourceGroup,
							Name:          ipRef.Name,
							Kind:          PublicIP,
						},
						Zones:   []string{zoneString},
						Managed: true,
					}
					ngw.PublicIPList = append(ngw.PublicIPList, ip)
				}
			} else {
				ip := PublicIPConfig{
					AzureResourceMetadata: AzureResourceMetadata{
						ResourceGroup: ia.ResourceGroup(),
						Name:          ia.publicIPName(ngw.Name),
						Kind:          PublicIP,
					},
					Managed:  false,
					Zones:    []string{zoneString},
					Location: ia.Region(),
				}
				ngw.PublicIPList = append(ngw.PublicIPList, ip)
			}
		}
		zones = append(zones, z)
	}

	return zones
}

func (ia *InfrastructureAdapter) defaultZone() []ZoneConfig {
	config := ia.config
	z := ZoneConfig{
		Subnet: SubnetConfig{
			AzureResourceMetadata: AzureResourceMetadata{
				ResourceGroup: ia.vnetConfig.ResourceGroup,
				Name:          ia.subnetName(nil),
				Parent:        ia.vnetConfig.Name,
				Kind:          Subnet,
			},
			cidr:            *config.Networks.Workers,
			serviceEndpoint: config.Networks.ServiceEndpoints,
		},
		Migrated: false,
	}
	if config.Networks.NatGateway == nil || !config.Networks.NatGateway.Enabled {
		return []ZoneConfig{z}
	}

	ngw := &NatGatewayConfig{
		AzureResourceMetadata: AzureResourceMetadata{
			ResourceGroup: ia.ResourceGroup(),
			Name:          ia.natGatewayName(),
			Kind:          NatGateway,
		},
		IdleTimeout: config.Networks.NatGateway.IdleConnectionTimeoutMinutes,
		Location:    ia.Region(),
	}
	if z := config.Networks.NatGateway.Zone; z != nil {
		ngw.Zone = to.Ptr(strconv.Itoa(int(*z)))
	}

	if len(config.Networks.NatGateway.IPAddresses) > 0 {
		for _, ipRef := range config.Networks.NatGateway.IPAddresses {
			ip := PublicIPConfig{
				AzureResourceMetadata: AzureResourceMetadata{
					ResourceGroup: ipRef.ResourceGroup,
					Name:          ipRef.Name,
					Kind:          PublicIP,
				},
				Managed: true,
			}
			ip.Zones = append(ip.Zones, strconv.Itoa(int(ipRef.Zone)))
			ngw.PublicIPList = append(ngw.PublicIPList, ip)
		}
	} else {
		ip := PublicIPConfig{
			AzureResourceMetadata: AzureResourceMetadata{
				ResourceGroup: ia.ResourceGroup(),
				Name:          ia.publicIPName(ngw.Name),
				Kind:          PublicIP,
			},
			Managed:  false,
			Location: ia.Region(),
		}
		if ngw.Zone != nil {
			ip.Zones = append(ip.Zones, *ngw.Zone)
		}
		ngw.PublicIPList = append(ngw.PublicIPList, ip)
	}
	z.NatGateway = ngw

	return []ZoneConfig{z}
}

func (ia *InfrastructureAdapter) ManagedIpConfigs() map[string]PublicIPConfig {
	res := make(map[string]PublicIPConfig)
	for _, z := range ia.zoneConfigs {
		if z.NatGateway == nil {
			continue
		}

		for _, ip := range z.NatGateway.PublicIPList {
			if !ip.Managed {
				res[ip.Name] = ip
			}
		}
	}

	return res
}

func (ia *InfrastructureAdapter) IpConfigs() []PublicIPConfig {
	var res []PublicIPConfig
	for _, z := range ia.zoneConfigs {
		if z.NatGateway == nil {
			continue
		}
		res = append(res, z.NatGateway.PublicIPList...)
	}

	return res
}

func (ia *InfrastructureAdapter) NatGatewayConfigs() map[string]NatGatewayConfig {
	res := make(map[string]NatGatewayConfig)
	for _, z := range ia.Zones() {
		if z.NatGateway != nil {
			res[z.NatGateway.Name] = *z.NatGateway
		}
	}

	return res
}

func (ia *InfrastructureAdapter) SubnetToNatMapping() map[string]string {
	res := map[string]string{}
	for _, z := range ia.Zones() {
		if z.NatGateway == nil {
			continue
		}
		res[z.Subnet.Name] = res[z.NatGateway.Name]
	}
	return res
}

// HasShootPrefix returns true if the target resource's name is prefixed with the shoot's canonical name.
func (ia *InfrastructureAdapter) HasShootPrefix(name *string) bool {
	if name == nil {
		return false
	}
	return strings.HasPrefix(*name, ia.TechnicalName())
}

func (ip *PublicIPConfig) ToProvider(base *armnetwork.PublicIPAddress) *armnetwork.PublicIPAddress {
	target := &armnetwork.PublicIPAddress{
		Location: to.Ptr(ip.Location),
		Properties: &armnetwork.PublicIPAddressPropertiesFormat{
			PublicIPAllocationMethod: to.Ptr(armnetwork.IPAllocationMethodStatic),
		},
		SKU: &armnetwork.PublicIPAddressSKU{
			Name: to.Ptr(armnetwork.PublicIPAddressSKUNameStandard),
			Tier: to.Ptr(armnetwork.PublicIPAddressSKUTierRegional),
		},
		Name: to.Ptr(ip.Name),
	}
	if len(ip.Zones) > 0 {
		// if no zones selected, zones has to be nil, to match what the API returns - otherwise reflect.DeepEqual fails the check.
		target.Zones = to.SliceOfPtrs(ip.Zones...)
	}

	// inherited from base
	if base != nil {
		target.ID = base.ID
		target.Tags = base.Tags
	}

	return target
}

func (nat *NatGatewayConfig) ToProvider(base *armnetwork.NatGateway) *armnetwork.NatGateway {
	target := &armnetwork.NatGateway{
		ID:       nil,
		Location: to.Ptr(nat.Location),
		Properties: &armnetwork.NatGatewayPropertiesFormat{
			IdleTimeoutInMinutes: nat.IdleTimeout,
		},
		SKU: &armnetwork.NatGatewaySKU{
			Name: to.Ptr(armnetwork.NatGatewaySKUNameStandard),
		},
		Name: to.Ptr(nat.Name),
	}
	if nat.Zone != nil {
		target.Zones = []*string{nat.Zone}
	}

	// inherited from base
	if base != nil {
		target.Properties.PublicIPPrefixes = base.Properties.PublicIPPrefixes
		target.ID = base.ID
	}
	return target
}

func (s *SubnetConfig) ToProvider(base *armnetwork.Subnet) *armnetwork.Subnet {
	target := &armnetwork.Subnet{
		Name: to.Ptr(s.Name),
		Properties: &armnetwork.SubnetPropertiesFormat{
			AddressPrefix: to.Ptr(s.cidr),
			// will be filled later
			NatGateway:           nil,
			NetworkSecurityGroup: nil,
			RouteTable:           nil,
		},
	}
	for _, endpoint := range s.serviceEndpoint {
		target.Properties.ServiceEndpoints = append(target.Properties.ServiceEndpoints, &armnetwork.ServiceEndpointPropertiesFormat{
			Service: to.Ptr(endpoint),
		})
	}

	// inherited from base
	if base != nil {
		target.ID = base.ID
		target.Properties.ServiceEndpointPolicies = base.Properties.ServiceEndpointPolicies
		target.Properties.PrivateLinkServiceNetworkPolicies = base.Properties.PrivateLinkServiceNetworkPolicies

		target.Properties.PrivateEndpoints = base.Properties.PrivateEndpoints
		target.Properties.PrivateEndpointNetworkPolicies = base.Properties.PrivateEndpointNetworkPolicies
		target.Properties.Delegations = base.Properties.Delegations
	}

	return target
}

func (v *VirtualNetworkConfig) ToProvider(base *armnetwork.VirtualNetwork) *armnetwork.VirtualNetwork {
	target := &armnetwork.VirtualNetwork{
		Location: to.Ptr(v.Location),
		Name:     to.Ptr(v.Name),
		Properties: &armnetwork.VirtualNetworkPropertiesFormat{
			AddressSpace: &armnetwork.AddressSpace{
				AddressPrefixes: []*string{v.CIDR},
			},
		},
	}
	if ddosId := v.DDoSPlan; ddosId != nil {
		target.Properties.EnableDdosProtection = to.Ptr(true)
		target.Properties.DdosProtectionPlan = &armnetwork.SubResource{ID: ddosId}
	} else {
		target.Properties.DdosProtectionPlan = nil
		target.Properties.EnableDdosProtection = to.Ptr(false)
	}

	if base != nil {
		target.Tags = base.Tags
		target.Properties.BgpCommunities = base.Properties.BgpCommunities
		target.Properties.EnableVMProtection = base.Properties.EnableVMProtection
		target.Properties.DhcpOptions = base.Properties.DhcpOptions
		target.Properties.Subnets = base.Properties.Subnets
		target.Properties.VirtualNetworkPeerings = base.Properties.VirtualNetworkPeerings
		target.Properties.FlowTimeoutInMinutes = base.Properties.FlowTimeoutInMinutes
		target.Properties.Encryption = base.Properties.Encryption
	}

	return target
}

func checkAllZonesWithFn[T any](t T, zones []ZoneConfig, check func(zone ZoneConfig, resource T) bool) bool {
	for _, n := range zones {
		if check(n, t) {
			return true
		}
	}
	return false
}
