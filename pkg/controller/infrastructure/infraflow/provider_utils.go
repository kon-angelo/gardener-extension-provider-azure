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
	"net/url"
	"reflect"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v4"
	"github.com/hashicorp/go-azure-helpers/resourcemanager/resourceids"
)

type AzureResourceKind string

const (
	VirtualNetwork  AzureResourceKind = "virtualNetworks"
	RouteTable      AzureResourceKind = "routeTables"
	SecurityGroup   AzureResourceKind = "networkSecurityGroups"
	NatGateway      AzureResourceKind = "natGateways"
	PublicIP        AzureResourceKind = "publicIPAddresses"
	Subnet          AzureResourceKind = "subnets"
	AvailabilitySet AzureResourceKind = "availabilitySets"
	ResourceGroup   AzureResourceKind = "resourceGroups"
)

const (
	PublicIPTemplate          = "/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/publicIPAddresses/%s"
	NatGatewayIdTemplate      = "/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/natGateways/%s"
	SecurityGroupTemplate     = "/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/networkSecurityGroups/%s"
	RouteTableTemplate        = "/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/routeTables/%s"
	AvailabilitySetIDTemplate = "/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Compute/availabilitySets/%s"
)

func GetIdFromTemplate(template, subscription, rgName, name string) string {
	return fmt.Sprintf(template, subscription, rgName, name)
}

type AzureResourceMetadata struct {
	ResourceGroup string
	Name          string
	Parent        string
	Kind          AzureResourceKind
}

// AzureResourceIdentifierFromID returns the identifier from parsing the object ID. It will always return a non-nil
// identifier if there was no error.
func AzureResourceIdentifierFromID(id string) (*AzureResourceMetadata, error) {
	rid, err := resourceids.ParseAzureResourceID(id)
	if err != nil {
		return nil, err
	}

	// the Name of the resource is not always there, especially in cases where they are referenced as subresources
	// from dependent resources.
	name, err := ResourceNameFromID(id)
	if err != nil {
		return nil, err
	}

	return &AzureResourceMetadata{
		ResourceGroup: rid.ResourceGroup,
		Name:          name,
	}, nil
}

// ResourceNameFromID returns the name of a resource based on its assigned ID.
func ResourceNameFromID(id string) (string, error) {
	idURL, err := url.ParseRequestURI(id)
	if err != nil {
		return "", err
	}

	path := idURL.Path

	path = strings.TrimPrefix(path, "/")
	path = strings.TrimSuffix(path, "/")

	components := strings.Split(path, "/")
	return components[len(components)-1], nil
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
