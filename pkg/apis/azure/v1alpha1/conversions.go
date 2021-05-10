// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package v1alpha1

import (
	"fmt"
	"unsafe"

	"k8s.io/apimachinery/pkg/conversion"
	"k8s.io/apimachinery/pkg/runtime"

	api "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
)

func addConversionFuncs(scheme *runtime.Scheme) error {
	return RegisterConversions(scheme)
}

func Convert_v1alpha1_InfrastructureConfig_To_azure_InfrastructureConfig(in *InfrastructureConfig, out *api.InfrastructureConfig, s conversion.Scope) error {
	out.ResourceGroup = (*api.ResourceGroup)(unsafe.Pointer(in.ResourceGroup))
	out.Identity = (*api.IdentityConfig)(unsafe.Pointer(in.Identity))

	if err := Convert_v1alpha1_To_Internal(&in.Networks, &out.Networks, in.Zoned, s); err != nil {
		return err
	}

	return nil
}

func Convert_azure_InfrastructureConfig_To_v1alpha1_InfrastructureConfig(in *api.InfrastructureConfig, out *InfrastructureConfig, s conversion.Scope) error {
	if err := autoConvert_azure_InfrastructureConfig_To_v1alpha1_InfrastructureConfig(in, out, s); err != nil {
		return err
	}

	if in.Networks.Regional == nil {
		out.Zoned = true
	}

	return nil
}

func Convert_v1alpha1_To_Internal(in *NetworkConfig, out *api.NetworkConfig, zoned bool, s conversion.Scope) error {
	if err := Convert_v1alpha1_VNet_To_azure_VNet(&in.VNet, &out.VNet, s); err != nil {
		return err
	}

	if !zoned {
		out.Regional = &api.RegionalTopology{
			CIDR:             in.Workers,
			ServiceEndpoints: in.ServiceEndpoints,
		}
		return nil
	}

	out.SingleSubnetZonal = &api.SingleSubnetZonalTopology{
		CIDR:            in.Workers,
		ServiceEndpoints: in.ServiceEndpoints,
	}

	if in.NatGateway == nil {
		return nil
	}

	natIn, natOut := &in.NatGateway, &out.SingleSubnetZonal.NatGateway
	*natOut = new(api.NatGatewayConfig)

	if err := Convert_v1alpha1_NatGatewayConfig_To_azure_NatGatewayConfig(*natIn, *natOut, s); err != nil {
		return err
	}

	return nil
}

func Convert_azure_NetworkConfig_To_v1alpha1_NetworkConfig(in *api.NetworkConfig, out *NetworkConfig, s conversion.Scope) error {
	if err := Convert_azure_VNet_To_v1alpha1_VNet(&in.VNet, &out.VNet, s); err != nil {
		return err
	}

	if in.Zonal != nil {
		return fmt.Errorf("cannot convert NetworkConfig using \"Zonal\" setup to v1alpha1")
	}

	if in.Regional != nil {
		out.Workers = in.Regional.CIDR
		out.ServiceEndpoints = in.Regional.ServiceEndpoints
		return nil
	}

	out.Workers = in.SingleSubnetZonal.CIDR
	out.ServiceEndpoints = in.SingleSubnetZonal.ServiceEndpoints

	if in.SingleSubnetZonal.NatGateway == nil {
		return nil
	}

	out.NatGateway = &NatGatewayConfig{}
	if err := Convert_azure_NatGatewayConfig_To_v1alpha1_NatGatewayConfig(in.SingleSubnetZonal.NatGateway, out.NatGateway, s); err != nil {
		return err
	}
	return nil
}

func Convert_azure_Subnet_To_v1alpha1_Subnet(in *api.Subnet, out *Subnet, s conversion.Scope) error {
	if err := autoConvert_azure_Subnet_To_v1alpha1_Subnet(in, out, s); err != nil {
		return err
	}
	return nil
}
