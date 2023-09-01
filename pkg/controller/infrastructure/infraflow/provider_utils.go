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

const (
	VirtualNetwork = "virtualNetwork"
	RouteTable     = "routeTable"
	SecurityGroup  = "securityGroup"
)

// HasGardenerPrefix returns true if the target Azure resource's name is prefixed with the shoot's canonical name.
// It can only be used in structs or pointers
func HasGardenerPrefix(item any, clusterName string) (bool, error) {
	v := reflect.ValueOf(item)

	if v.Kind() == reflect.Pointer {
		v = v.Elem()
	}

	if v.Kind() != reflect.Struct {
		return false, fmt.Errorf("HasGardenerPrefix can only be used on pointers and structs")
	}

	f := v.FieldByName("Name")
	if !f.IsValid() {
		return false, fmt.Errorf("field: name was not found")
	}

	if f.IsNil() {
		return false, nil
	}

	return strings.HasPrefix(f.Elem().String(), clusterName), nil
}

type AzureResourceIdentifier struct {
	ResourceGroup string
	Name          string
	Kind          string
}

// AzureResourceIdentifierFromID returns the identifier from parsing the object ID. It will always return a non-nil
// identifier if there was no error.
func AzureResourceIdentifierFromID(id string) (*AzureResourceIdentifier, error) {
	rid, err := resourceids.ParseAzureResourceID(id)
	if err != nil {
		return nil, err
	}

	// the Name of the resource is not always there, especially in cases where they are referenced as subresources
	// from dependent resources.
	name, err := ResourceName(id)
	if err != nil {
		return nil, err
	}

	return &AzureResourceIdentifier{
		ResourceGroup: rid.ResourceGroup,
		Name:          name,
	}, nil
}

func defaultNatName(clusterName string) string {
	return fmt.Sprintf("%s-nat-gateway", clusterName)
}

func defaultNatIPName(clusterName string) string {
	return fmt.Sprintf("%s-ip", defaultNatName(clusterName))
}

func ResourceName(id string) (string, error) {
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

// type aliases

// PublicIPAddress is an alias for the real pip type.
type PublicIPAddress armnetwork.PublicIPAddress

// Compare public current and target spec between PIPs and decide if they need to be deleted.
func (p PublicIPAddress) Compare(target PublicIPAddress) bool {
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

// Subnet is an alias for the real subnet type.
type Subnet armnetwork.Subnet

// ResourceIdentifier returns the Identifier for this resource.
func (s Subnet) ResourceIdentifier() (*AzureResourceIdentifier, error) {
	if s.ID == nil {
		return nil, fmt.Errorf("can't create azure identifier when ID is missing")
	}

	return AzureResourceIdentifierFromID(*s.ID)
}
