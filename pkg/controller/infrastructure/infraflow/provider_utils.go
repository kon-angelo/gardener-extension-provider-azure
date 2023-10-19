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
	"reflect"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v4"
)

type AzureResourceKind string

func (a AzureResourceKind) String() string {
	return string(a)
}

const (
	KindAvailabilitySet AzureResourceKind = "Microsoft.Compute/availabilitySets"
	KindNatGateway      AzureResourceKind = "Microsoft.Network/natGateways"
	KindPublicIP        AzureResourceKind = "Microsoft.Network/publicIPAddresses"
	KindResourceGroup   AzureResourceKind = "Microsoft.Resources/resourceGroups"
	KindRouteTable      AzureResourceKind = "Microsoft.Network/routeTables"
	KindSecurityGroup   AzureResourceKind = "Microsoft.Network/networkSecurityGroups"
	KindSubnet          AzureResourceKind = "Microsoft.Network/virtualNetworks/subnets"
	KindVirtualNetwork  AzureResourceKind = "Microsoft.Network/virtualNetworks"
)

var ResourceCatalog = []AzureResourceKind{
	KindAvailabilitySet,
	KindNatGateway,
	KindPublicIP,
	KindResourceGroup,
	KindRouteTable,
	KindSecurityGroup,
	KindSubnet,
	KindVirtualNetwork,
}

const (
	TemplateAvailabilitySet = "/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Compute/availabilitySets/%s"
	TemplateNatGateway      = "/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/natGateways/%s"
	TemplatePublicIP        = "/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/publicIPAddresses/%s"
	TemplateResourceGroup   = "/subscriptions/%s/resourceGroups/%s"
	TemplateRouteTable      = "/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/routeTables/%s"
	TemplateSecurityGroup   = "/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/networkSecurityGroups/%s"
)

func ResourceGroupIdFromTemplate(subscription, name string) string {
	return fmt.Sprintf(TemplateResourceGroup, subscription, name)
}

func GetIdFromTemplate(template, subscription, rgName, name string) string {
	return fmt.Sprintf(template, subscription, rgName, name)
}

type AzureResourceMetadata struct {
	ResourceGroup string
	Name          string
	Parent        string
	Kind          AzureResourceKind
}

// ForceNewIp checks if the resource can be reconciled. If not, returns the offender's name.
func ForceNewIp(current, target *armnetwork.PublicIPAddress) (bool, string, any) {
	if !reflect.DeepEqual(current.Location, target.Location) {
		return true, "Location", *current.Location
	}
	if !reflect.DeepEqual(current.Zones, target.Zones) {
		return true, "Zones", current.Zones
	}
	if !reflect.DeepEqual(current.Properties.PublicIPAllocationMethod, target.Properties.PublicIPAllocationMethod) {
		return true, "PublicIPAllocationMethod", current.Properties.PublicIPAllocationMethod
	}
	return false, "", nil
}

func ForceNewNat(current, target *armnetwork.NatGateway) (bool, string, any) {
	if !reflect.DeepEqual(current.Location, target.Location) {
		return true, "Location", *current.Location
	}
	if !reflect.DeepEqual(current.Zones, target.Zones) {
		return true, "Zones", current.Zones
	}

	return false, "", nil
}

func ForceNewSubnet(current, target *armnetwork.Subnet) (bool, string, any) {
	return false, "", nil
}
