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
	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"

	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/helper"
	consts "github.com/gardener/gardener-extension-provider-azure/pkg/azure"
	"github.com/gardener/gardener-extension-provider-azure/pkg/internal/infrastructure"
)

// InfrastructureAdapter contains information about the infrastructure resources that are either static, or otherwise
// inferable based on the shoot configuration.
type InfrastructureAdapter struct {
	infra   *extensionsv1alpha1.Infrastructure
	config  *azure.InfrastructureConfig
	status  *azure.InfrastructureStatus
	profile *azure.CloudProfileConfig
	cluster *extensionscontroller.Cluster
}

// ClusterName the cluster's "base" name. Used as a name or as a prefix by other resources.
func (i *InfrastructureAdapter) ClusterName() string {
	return i.infra.Namespace
}

// ResourceGroupName the name of the resource group.
func (i *InfrastructureAdapter) ResourceGroupName() string {
	return i.ClusterName()
}

// GardenerVnet returns true if gardener manages the shoot's virtual network.
func (i *InfrastructureAdapter) GardenerVnet() bool {
	return i.config.Networks.VNet.ResourceGroup == nil
}

// Region is the region of the shoot.
func (i *InfrastructureAdapter) Region() string {
	return i.infra.Spec.Region
}

// VnetName the name of the shoot's virtual network.
func (i *InfrastructureAdapter) VnetName() string {
	if i.GardenerVnet() {
		return i.ClusterName()
	}

	return *i.config.Networks.VNet.Name
}

// VnetResourceGroup is the virtual network's resource group.
func (i *InfrastructureAdapter) VnetResourceGroup() string {
	if i.GardenerVnet() {
		return i.ResourceGroupName()
	}

	return *i.config.Networks.VNet.ResourceGroup
}

// AvailabilitySetConfig contains the configuration for the shoot's availability set.
type AvailabilitySetConfig struct {
	CountFaultDomains  *int32
	CountUpdateDomains *int32
}

// AvailabilitySetRequired returns true if gardener should create an availability set for the shoot.
func (i *InfrastructureAdapter) AvailabilitySetRequired() (bool, error) {
	return infrastructure.IsPrimaryAvailabilitySetRequired(i.infra, i.config, i.cluster)
}

// AvailabilitySetName the name of the availability set.
func (i *InfrastructureAdapter) AvailabilitySetName() string {
	return fmt.Sprintf("%s-avset-workers", i.ClusterName())

}

// AvailabilitySet returns the availability set's configuration.
func (i *InfrastructureAdapter) AvailabilitySet() (*AvailabilitySetConfig, error) {
	asc := &AvailabilitySetConfig{}

	if i.status != nil {
		nodesAVSet, err := helper.FindAvailabilitySetByPurpose(i.status.AvailabilitySets, azure.PurposeNodes)
		if err != nil {
			return nil, fmt.Errorf("error obtaining update and fault domain counts from infrastructure status: %v", err)
		}
		asc.CountFaultDomains = nodesAVSet.CountFaultDomains
		asc.CountUpdateDomains = nodesAVSet.CountUpdateDomains
	}

	if asc.CountFaultDomains == nil {
		count, err := helper.FindDomainCountByRegion(i.profile.CountFaultDomains, i.Region())
		if err != nil {
			return nil, err
		}
		asc.CountFaultDomains = to.Ptr(count)
	}
	if asc.CountUpdateDomains == nil {
		count, err := helper.FindDomainCountByRegion(i.profile.CountUpdateDomains, i.Region())
		if err != nil {
			return nil, err
		}
		asc.CountUpdateDomains = to.Ptr(count)
	}

	return asc, nil
}

// RouteTableName returns the name of the shoot's route table.
func (i *InfrastructureAdapter) RouteTableName() string {
	return "worker_route_table"
}

// SecurityGroupName returns the name of the shoot's security group.
func (i *InfrastructureAdapter) SecurityGroupName() string {
	return fmt.Sprintf("%s-workers", i.ClusterName())
}

func (i *InfrastructureAdapter) NatGatewayName() string {
	return fmt.Sprintf("%s-nat-gateway", i.ClusterName())
}

func (i *InfrastructureAdapter) SubnetName(zone *int32) string {
	n := fmt.Sprintf("%s-nodes", i.ClusterName())
	if zone != nil {
		n = fmt.Sprintf("%s-z%d", n, *zone)
	}
	return n
}

func (i *InfrastructureAdapter) NatGatewayNameForZone(zone int32, migrated bool) string {
	if migrated {
		return i.NatGatewayName()
	}

	return fmt.Sprintf("%s-z%d", i.NatGatewayName(), zone)
}

func (i *InfrastructureAdapter) PublicIPName(natName string) string {
	return fmt.Sprintf("%s-ip", natName)
}

type publicIP struct {
	AzureResourceIdentifier
	zones       []string
	userManaged bool
}

type natGateway struct {
	AzureResourceIdentifier
	zone        *string
	idleTimeout *int32
	pip         []publicIP
}

type subnet struct {
	AzureResourceIdentifier
	cidr            string
	serviceEndpoint []string
}

type zone struct {
	subnet     subnet
	natGateway *natGateway
	migrated   bool
}

func (i *InfrastructureAdapter) Zones() []zone {
	if len(i.config.Networks.Zones) == 0 {
		return i.DefaultZone()
	}

	var zones []zone
	migratedZone, ok := i.infra.Annotations[consts.NetworkLayoutZoneMigrationAnnotation]
	for _, configZone := range i.config.Networks.Zones {
		zoneString := helper.InfrastructureZoneToString(configZone.Name)
		isMigratedZone := ok && migratedZone == zoneString
		z := zone{
			subnet: subnet{
				AzureResourceIdentifier: AzureResourceIdentifier{
					ResourceGroup: i.VnetResourceGroup(),
					Name:          i.SubnetName(&configZone.Name),
					Kind:          Subnet,
				},
				cidr:            configZone.CIDR,
				serviceEndpoint: configZone.ServiceEndpoints,
			},
			migrated: isMigratedZone,
		}

		if configZone.NatGateway != nil && configZone.NatGateway.Enabled {
			ngw := &natGateway{
				AzureResourceIdentifier: AzureResourceIdentifier{
					ResourceGroup: i.ResourceGroupName(),
					Name:          i.NatGatewayNameForZone(configZone.Name, isMigratedZone),
					Kind:          NatGateway,
				},
				idleTimeout: configZone.NatGateway.IdleConnectionTimeoutMinutes,
			}
			ngw.zone = to.Ptr(zoneString)

			if len(configZone.NatGateway.IPAddresses) > 0 {
				for _, ipRef := range configZone.NatGateway.IPAddresses {
					ip := publicIP{
						AzureResourceIdentifier: AzureResourceIdentifier{
							ResourceGroup: ipRef.ResourceGroup,
							Name:          ipRef.Name,
							Kind:          PublicIP,
						},
						zones:       []string{zoneString},
						userManaged: true,
					}
					ngw.pip = append(ngw.pip, ip)
				}
			} else {
				ip := publicIP{
					AzureResourceIdentifier: AzureResourceIdentifier{
						ResourceGroup: i.ResourceGroupName(),
						Name:          i.PublicIPName(ngw.Name),
						Kind:          PublicIP,
					},
					userManaged: false,
					zones:       []string{zoneString},
				}
				ngw.pip = append(ngw.pip, ip)
			}
			z.natGateway = ngw
		}
		zones = append(zones, z)
	}

	return zones
}

func (i *InfrastructureAdapter) DefaultZone() []zone {
	config := i.config
	z := zone{
		subnet: subnet{
			AzureResourceIdentifier: AzureResourceIdentifier{
				ResourceGroup: i.VnetResourceGroup(),
				Name:          i.SubnetName(nil),
				Kind:          Subnet,
			},
			cidr:            *config.Networks.Workers,
			serviceEndpoint: config.Networks.ServiceEndpoints,
		},
		migrated: false,
	}
	if config.Networks.NatGateway == nil || !config.Networks.NatGateway.Enabled {
		return []zone{z}
	}

	ngw := &natGateway{
		AzureResourceIdentifier: AzureResourceIdentifier{
			ResourceGroup: i.ResourceGroupName(),
			Name:          i.NatGatewayName(),
			Kind:          NatGateway,
		},
		idleTimeout: config.Networks.NatGateway.IdleConnectionTimeoutMinutes,
	}
	if z := config.Networks.NatGateway.Zone; z != nil {
		ngw.zone = to.Ptr(strconv.Itoa(int(*z)))
	}

	if len(config.Networks.NatGateway.IPAddresses) > 0 {
		for _, ipRef := range config.Networks.NatGateway.IPAddresses {
			ip := publicIP{
				AzureResourceIdentifier: AzureResourceIdentifier{
					ResourceGroup: ipRef.ResourceGroup,
					Name:          ipRef.Name,
					Kind:          PublicIP,
				},
				userManaged: true,
			}
			ip.zones = append(ip.zones, strconv.Itoa(int(ipRef.Zone)))
			ngw.pip = append(ngw.pip, ip)
		}
	} else {
		ip := publicIP{
			AzureResourceIdentifier: AzureResourceIdentifier{
				ResourceGroup: i.ResourceGroupName(),
				Name:          i.PublicIPName(ngw.Name),
				Kind:          PublicIP,
			},
			userManaged: false,
		}
		if ngw.zone != nil {
			ip.zones = append(ip.zones, *ngw.zone)
		}
		ngw.pip = append(ngw.pip, ip)
	}
	z.natGateway = ngw

	return []zone{z}
}

func (i *InfrastructureAdapter) IPs() []publicIP {
	var res []publicIP
	for _, z := range i.Zones() {
		if z.natGateway == nil {
			continue
		}
		res = append(res, z.natGateway.pip...)
	}

	return res
}

func (i *InfrastructureAdapter) Nats() []natGateway {
	var res []natGateway
	for _, z := range i.Zones() {
		if z.natGateway != nil {
			res = append(res, *z.natGateway)
		}
	}

	return res
}

func (i *InfrastructureAdapter) SubnetToNatMapping() map[string]string {
	res := map[string]string{}
	for _, z := range i.Zones() {
		if z.natGateway == nil {
			continue
		}
		res[z.subnet.Name] = res[z.natGateway.Name]
	}
	return res
}

func (i *InfrastructureAdapter) NatToIPMapping() map[string]string {
	return nil
}

// HasShootPrefix returns true if the target resource's name is prefixed with the shoot's canonical name.
func (i *InfrastructureAdapter) HasShootPrefix(name *string) bool {
	if name == nil {
		return false
	}
	return strings.HasPrefix(*name, i.ClusterName())
}
