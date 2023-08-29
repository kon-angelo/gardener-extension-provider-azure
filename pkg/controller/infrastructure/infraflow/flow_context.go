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

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v2"
	"github.com/gardener/gardener/extensions/pkg/controller"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/utils/flow"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/helper"
	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/v1alpha1"
	"github.com/gardener/gardener-extension-provider-azure/pkg/azure/client"
	"github.com/gardener/gardener-extension-provider-azure/pkg/controller/infrastructure/infraflow/shared"
)

// Key names for the whiteboard object to pass results between the reconcilation tasks
const (
	routeTableID  = "route_table_id"
	sGroupID      = "security_group_id"
	natGatewayMap = "nategateway_map"
	publicIPMap   = "public_ip_map"
)

// FlowContext is the reconciler for all managed resources
type FlowContext struct {
	*shared.BasicFlowContext
	logger logr.Logger

	cfg        *azure.InfrastructureConfig
	factory    client.Factory
	infra      *extensionsv1alpha1.Infrastructure
	cluster    *controller.Cluster
	whiteboard shared.Whiteboard
	tf         *TerraformAdapter
	provider   CloudProvider
}

// NewFlowContext creates a new FlowContext.
func NewFlowContext(factory client.Factory, logger logr.Logger, infra *extensionsv1alpha1.Infrastructure, cluster *controller.Cluster) (*FlowContext, error) {
	wb := shared.NewWhiteboard()
	bfc := shared.NewBasicFlowContext(logger, wb, nil)

	cfg, err := helper.InfrastructureConfigFromInfrastructure(infra)
	if err != nil {
		return nil, err
	}

	tfAdapter, err := NewTerraformAdapter(infra, cfg, cluster)
	if err != nil {
		return nil, err
	}

	return &FlowContext{
		BasicFlowContext: bfc,
		factory:          factory,
		logger:           logger,
		infra:            infra,
		cluster:          cluster,
		cfg:              cfg,
		whiteboard:       wb,
		tf:               tfAdapter,
		provider:         &provider{factory},
	}, nil
}

// Reconcile reconciles all resources
func (f *FlowContext) Reconcile(ctx context.Context) (*v1alpha1.InfrastructureStatus, *runtime.RawExtension, error) {
	graph := f.buildReconcileGraph()
	fl := graph.Compile()
	if err := fl.Run(ctx, flow.Opts{}); err != nil {
		return nil, nil, err
	}
	status, err1 := f.GetInfrastructureStatus(ctx)
	state, err2 := f.GetInfrastructureState()
	err := errors.Join(err1, err2)
	return status, state, err
}

func (f *FlowContext) buildReconcileGraph() *flow.Graph {
	g := flow.NewGraph("Azure infrastructure reconciliation")
	resourceGroup := f.AddTask(g, "ensure resource group", f.EnsureResourceGroup)
	vnet := f.AddTask(g, "ensure vnet", f.EnsureVnet, shared.Dependencies(resourceGroup))
	f.AddTask(g, "ensure availability set", f.EnsureAvailabilitySet, shared.DoIf(!f.cfg.Zoned), shared.Dependencies(resourceGroup))
	routeTable := f.AddTask(g, "ensure route table", f.EnsureRouteTable, shared.Dependencies(resourceGroup))
	securityGroup := f.AddTask(g, "ensure security group", f.EnsureSecurityGroup, shared.Dependencies(resourceGroup))

	ip := f.AddTask(g, "ensure pips", f.EnsurePublicIPs2(ctx), shared.Dependencies(resourceGroup))
	natGateway := f.AddTask(g, "ensure nat gateway", func(ctx context.Context) error {
		return nil
	}
	// ip := f.AddTask(g, "ensure pips", func(ctx context.Context) error {
	// 	ips, err := f.EnsurePublicIPs(ctx)
	// 	if err != nil {
	// 		return err
	// 	}
	// 	err = f.EnrichResponseWithUserManagedIPs(ctx, ips)
	// 	if err != nil {
	// 		return fmt.Errorf("enrichment with user managed IPs failed: %v", err)
	// 	}
	// 	f.whiteboard.SetObject(publicIPMap, ips)
	// 	return nil
	// }, shared.Dependencies(resourceGroup))

	// natGateway := f.AddTask(g, "ensure nat gateway", func(ctx context.Context) error {
	// 	ips := f.whiteboard.GetObject(publicIPMap).(map[string][]*armnetwork.PublicIPAddress)
	// 	resp, err := f.EnsureNatGateways(ctx, ips)
	// 	f.whiteboard.SetObject(natGatewayMap, resp)
	// 	return err
	// }, shared.Dependencies(ip))

	f.AddTask(g, "ensure subnet", func(ctx context.Context) error {
		routeTable := armnetwork.RouteTable{
			ID: f.whiteboard.Get(routeTableID),
		}
		securityGroup := armnetwork.SecurityGroup{
			ID: f.whiteboard.Get(sGroupID),
		}
		natGateway := f.whiteboard.GetObject(natGatewayMap).(map[string]*armnetwork.NatGateway)
		return f.EnsureSubnets(ctx, securityGroup, routeTable, natGateway)
	}, shared.Dependencies(securityGroup), shared.Dependencies(routeTable), shared.Dependencies(natGateway), shared.Dependencies(vnet))
	return g
}

// Delete deletes all resources managed by the reconciler
func (f *FlowContext) Delete(ctx context.Context) error {
	graph := flow.NewGraph("Azure infrastructure deletion")
	foreignSubnets := f.AddTask(graph, "delete subnets in foreign resource group", f.deleteSubnetsInForeignGroup)
	f.AddTask(graph, "delete resource group", f.DeleteResourceGroup, shared.Dependencies(foreignSubnets))
	fl := graph.Compile()
	if err := fl.Run(ctx, flow.Opts{}); err != nil {
		return flow.Causes(err)
	}
	return nil
}

func (f *FlowContext) ensureObjectKeys(keys ...string) error {
	for _, k := range keys {
		if f.whiteboard.GetObject(k) == nil {
			return fmt.Errorf("could not locate required key: %s", k)
		}
	}
	return nil
}
