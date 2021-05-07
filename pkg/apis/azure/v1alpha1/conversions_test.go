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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/runtime"

	api "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
)

var _ = Describe("Conversions", func() {

	var scheme *runtime.Scheme

	BeforeSuite(func() {
		scheme = runtime.NewScheme()
		Expect(AddToScheme(scheme)).NotTo(HaveOccurred())
	})

	Context("#Infrastructure conversions", func() {
		Context("#Convert_v1alpha1_InfrastructureConfig_To_azure_InfrastructureConfig", func() {
			Describe("Regional", func() {
				var (
					cidr      = "1.1.1.1/24"
					endpoints = []string{"ep1", "ep2"}

					out = &api.InfrastructureConfig{}
					in  = &InfrastructureConfig{
						Networks: NetworkConfig{
							VNet: VNet{
								CIDR: &cidr,
							},
							Workers:          cidr,
							ServiceEndpoints: endpoints,
						},
					}
				)

				It("#should correctly convert", func() {
					Expect(scheme.Convert(in, out, nil)).NotTo(HaveOccurred())
					Expect(out).To(Equal(&api.InfrastructureConfig{
						Networks: api.NetworkConfig{
							VNet: api.VNet{
								CIDR: &cidr,
							},
							Topology: api.Topology{
								Regional: &api.RegionalTopology{
									CIDR:             cidr,
									ServiceEndpoints: endpoints,
								},
							},
						},
					}))
				})
			})

			Describe("SingleSubnetZonal", func() {
				var (
					cidr              = "1.1.1.1/24"
					endpoints         = []string{"ep1", "ep2"}
					zone        int32 = 2
					idleTimeout int32 = 5

					out    = &api.InfrastructureConfig{}
					config = &InfrastructureConfig{
						Zoned: true,
						Networks: NetworkConfig{
							VNet: VNet{
								CIDR: &cidr,
							},
							Workers:          cidr,
							ServiceEndpoints: endpoints,
						},
					}

					configWithNat = &InfrastructureConfig{
						Zoned: true,
						Networks: NetworkConfig{
							VNet: VNet{
								CIDR: &cidr,
							},
							Workers:          cidr,
							ServiceEndpoints: endpoints,
							NatGateway: &NatGatewayConfig{
								Enabled:                      true,
								Zone:                         &zone,
								IdleConnectionTimeoutMinutes: &idleTimeout,
							},
						},
					}
				)

				It("#should correctly convert", func() {
					Expect(scheme.Convert(config, out, nil)).NotTo(HaveOccurred())
					Expect(out).To(Equal(&api.InfrastructureConfig{
						Networks: api.NetworkConfig{
							VNet: api.VNet{
								CIDR: &cidr,
							},
							Topology: api.Topology{
								SingleSubnetZonal: &api.SingleSubnetZonalTopology{
									CIDR:             cidr,
									ServiceEndpoints: endpoints,
									NatGateway:       nil,
								},
							},
						},
					}))
				})

				It("#should correctly convert config with NATGateway", func() {
					Expect(scheme.Convert(configWithNat, out, nil)).NotTo(HaveOccurred())
					Expect(out).To(Equal(&api.InfrastructureConfig{
						Networks: api.NetworkConfig{
							VNet: api.VNet{
								CIDR: &cidr,
							},
							Topology: api.Topology{
								SingleSubnetZonal: &api.SingleSubnetZonalTopology{
									CIDR:             cidr,
									ServiceEndpoints: endpoints,
									NatGateway: &api.NatGatewayConfig{
										Enabled:                      true,
										Zone:                         &zone,
										IdleConnectionTimeoutMinutes: &idleTimeout,
									},
								},
							},
						},
					}))
				})
			})
		})

		Describe("#Convert_azure_InfrastructureConfig_To_v1alpha1_InfrastructureConfig", func() {
			Context("Regional", func(){
				var (
					cidr      = "1.1.1.1/24"
					endpoints = []string{"ep1", "ep2"}
					in        = &api.InfrastructureConfig{
						Networks: api.NetworkConfig{
							VNet: api.VNet{
								CIDR: &cidr,
							},
							Topology: api.Topology{
								Regional: &api.RegionalTopology{
									CIDR:             cidr,
									ServiceEndpoints: endpoints,
								},
							},
						},
					}
					out = &InfrastructureConfig{}
				)

				It("#should correctly convert", func() {
					Expect(scheme.Convert(in, out, nil)).NotTo(HaveOccurred())
					Expect(out).To(Equal(&InfrastructureConfig{
						Networks: NetworkConfig{
							VNet: VNet{
								CIDR: &cidr,
							},
							Workers:          cidr,
							ServiceEndpoints: endpoints,
						},
						Zoned: false,
					}))
				})
			})

			Context("SingleSubnetZonal", func(){
				var (
					cidr      = "1.1.1.1/24"
					endpoints = []string{"ep1", "ep2"}
					zone int32 = 2
					config = &api.InfrastructureConfig{
						Networks: api.NetworkConfig{
							VNet: api.VNet{
								CIDR: &cidr,
							},
							Topology: api.Topology{
								SingleSubnetZonal: &api.SingleSubnetZonalTopology{
									CIDR:             cidr,
									ServiceEndpoints: endpoints,
								},
							},
						},
					}
					configWithNat = &api.InfrastructureConfig{
						Networks: api.NetworkConfig{
							VNet: api.VNet{
								CIDR: &cidr,
							},
							Topology: api.Topology{
								SingleSubnetZonal: &api.SingleSubnetZonalTopology{
									CIDR:             cidr,
									ServiceEndpoints: endpoints,
									NatGateway: &api.NatGatewayConfig{
										Enabled:                      true,
										Zone:                         &zone,
									},
								},
							},
						},
					}
					out = &InfrastructureConfig{}
				)

				It("#should correctly convert", func() {
					Expect(scheme.Convert(config, out, nil)).NotTo(HaveOccurred())
					Expect(out).To(Equal(&InfrastructureConfig{
						Networks: NetworkConfig{
							VNet: VNet{
								CIDR: &cidr,
							},
							Workers:          cidr,
							ServiceEndpoints: endpoints,
						},
						Zoned: true,
					}))
				})

				It("#should correctly convert - NATGateway", func() {
					Expect(scheme.Convert(configWithNat, out, nil)).NotTo(HaveOccurred())
					Expect(out).To(Equal(&InfrastructureConfig{
						Networks: NetworkConfig{
							VNet: VNet{
								CIDR: &cidr,
							},
							Workers:          cidr,
							ServiceEndpoints: endpoints,
							NatGateway: &NatGatewayConfig{
								Enabled:                      true,
								Zone:                         &zone,
							},
						},
						Zoned: true,
					}))
				})
			})
		})
	})
})
