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
	. "github.com/onsi/ginkgo/extensions/table"
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

	Context("#InfrastructureConfig", func() {
		var (
			cidr              = "1.1.1.1/24"
			endpoints         = []string{"ep1", "ep2"}
			zone        int32 = 2
			idleTimeout int32 = 5
		)

		DescribeTable("Convert_v1alpha1_InfrastructureConfig_To_azure_InfrastructureConfig", func(in *InfrastructureConfig, expected *api.InfrastructureConfig) {
			out := &api.InfrastructureConfig{}
			err := scheme.Convert(in, out, nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(out).To(Equal(expected))
		},
			Entry("should succeed - regional",
				&InfrastructureConfig{
					Networks: NetworkConfig{
						VNet: VNet{
							CIDR: &cidr,
						},
						Workers:          cidr,
						ServiceEndpoints: endpoints,
					},
				}, &api.InfrastructureConfig{
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
				}),
				Entry("should succeed - singleSubnetZonal without NAT",
					&InfrastructureConfig{
						Zoned: true,
						Networks: NetworkConfig{
							VNet: VNet{
								CIDR: &cidr,
							},
							Workers:          cidr,
							ServiceEndpoints: endpoints,
						},
					},&api.InfrastructureConfig{
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
					}),
			Entry("should succeed - singleSubnetZonal with NAT",
				&InfrastructureConfig{
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
						},
					},
				},&api.InfrastructureConfig{
					Networks: api.NetworkConfig{
						VNet: api.VNet{
							CIDR: &cidr,
						},
						Topology: api.Topology{
							SingleSubnetZonal: &api.SingleSubnetZonalTopology{
								CIDR:             cidr,
								ServiceEndpoints: endpoints,
								NatGateway:       &api.NatGatewayConfig{
									Enabled:                      true,
									Zone:                         &zone,
								},
							},
						},
					},
				}),
		)

		DescribeTable("#Convert_azure_InfrastructureConfig_To_v1alpha1_InfrastructureConfig", func(in *api.InfrastructureConfig, expected *InfrastructureConfig, expError bool) {
			out := &InfrastructureConfig{}
			err := scheme.Convert(in, out, nil)
			if expError {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
				Expect(out).To(Equal(expected))
			}
		},
			Entry("should succeed - regional",
				&api.InfrastructureConfig{
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
				}, &InfrastructureConfig{
					Networks: NetworkConfig{
						VNet: VNet{
							CIDR: &cidr,
						},
						Workers:          cidr,
						ServiceEndpoints: endpoints,
					},
					Zoned: false,
				}, false),
			Entry("should succeed - singleSubnetZonal without NAT",
				&api.InfrastructureConfig{
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
				}, &InfrastructureConfig{
					Networks: NetworkConfig{
						VNet: VNet{
							CIDR: &cidr,
						},
						Workers:          cidr,
						ServiceEndpoints: endpoints,
					},
					Zoned: true,
				}, false),
			Entry("should succeed - singleSubnetZonal with NAT enabled",
				&api.InfrastructureConfig{
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
									IdleConnectionTimeoutMinutes: &idleTimeout,
									Zone:                         &zone,
								},
							},
						},
					},
				},
				&InfrastructureConfig{
					Networks: NetworkConfig{
						VNet: VNet{
							CIDR: &cidr,
						},
						Workers:          cidr,
						ServiceEndpoints: endpoints,
						NatGateway: &NatGatewayConfig{
							Enabled:                      true,
							IdleConnectionTimeoutMinutes: &idleTimeout,
							Zone:                         &zone,
						},
					},
					Zoned: true,
				}, false),
			Entry("singleSubnetRegional - NAT disabled",
				&api.InfrastructureConfig{
					Networks: api.NetworkConfig{
						VNet: api.VNet{
							CIDR: &cidr,
						},
						Topology: api.Topology{
							SingleSubnetZonal: &api.SingleSubnetZonalTopology{
								CIDR:             cidr,
								ServiceEndpoints: endpoints,
								NatGateway: &api.NatGatewayConfig{
									Enabled:                      false,
									IdleConnectionTimeoutMinutes: &idleTimeout,
									Zone:                         &zone,
								},
							},
						},
					},
				},
				&InfrastructureConfig{
					Networks: NetworkConfig{
						VNet: VNet{
							CIDR: &cidr,
						},
						Workers:          cidr,
						ServiceEndpoints: endpoints,
						NatGateway: &NatGatewayConfig{
							Enabled:                      false,
							IdleConnectionTimeoutMinutes: &idleTimeout,
							Zone:                         &zone,
						},
					},
					Zoned: true,
				}, false),
			Entry("should fail converting Zonal setup",
				&api.InfrastructureConfig{
					Networks: api.NetworkConfig{
						VNet: api.VNet{
							CIDR: &cidr,
						},
						Topology: api.Topology{
							Zonal: &api.ZonalTopology{
								Zones: []api.Zone{
									{Name: zone, CIDR: cidr},
								},
							},
						},
					},
				}, nil, true),
		)
	})
})
