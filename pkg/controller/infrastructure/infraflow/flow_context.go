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
	"github.com/gardener/gardener-extension-provider-azure/pkg/internal"
	"github.com/gardener/gardener-extension-provider-azure/pkg/internal/infrastructure"
)

// FlowContext is the reconciler for all managed resources
type FlowContext struct {
	*shared.BasicFlowContext
	logger logr.Logger

	persistFunc func(extension *runtime.RawExtension) error
	cfg         *azure.InfrastructureConfig
	factory     client.Factory
	auth        *internal.ClientAuth
	infra       *extensionsv1alpha1.Infrastructure
	cluster     *controller.Cluster
	whiteboard  shared.Whiteboard
	adapter     *InfrastructureAdapter
	provider    Access
	state       *azure.InfrastructureState
}

// NewFlowContext creates a new FlowContext.
func NewFlowContext(factory client.Factory,
	auth *internal.ClientAuth,
	logger logr.Logger,
	infra *extensionsv1alpha1.Infrastructure,
	cluster *controller.Cluster,
	state *azure.InfrastructureState,
	persistFunc func(extension *runtime.RawExtension) error,
) (*FlowContext, error) {
	wb := shared.NewWhiteboard()
	for k, v := range state.Data {
		wb.Set(k, v)
	}

	bfc := shared.NewBasicFlowContext(logger, wb)
	cfg, err := helper.InfrastructureConfigFromInfrastructure(infra)
	if err != nil {
		return nil, err
	}

	profile, err := helper.CloudProfileConfigFromCluster(cluster)
	if err != nil {
		return nil, err
	}

	var status *azure.InfrastructureStatus
	if infra.Status.ProviderStatus != nil {
		status, err = helper.InfrastructureStatusFromRaw(infra.Status.ProviderStatus)
		if err != nil {
			return nil, err
		}
	}

	adapter, err := NewInfrastructureAdapter(
		infra,
		cfg,
		status,
		profile,
		cluster,
	)
	if err != nil {
		return nil, err
	}

	return &FlowContext{
		BasicFlowContext: bfc,
		factory:          factory,
		auth:             auth,
		logger:           logger,
		infra:            infra,
		state:            state,
		cluster:          cluster,
		cfg:              cfg,
		whiteboard:       wb,
		persistFunc:      persistFunc,
		provider: &access{
			factory,
		},
		adapter: adapter,
	}, nil
}

// Reconcile reconciles all resources
func (f *FlowContext) Reconcile(ctx context.Context) (*v1alpha1.InfrastructureStatus, *runtime.RawExtension, error) {
	graph := f.buildReconcileGraph()
	fl := graph.Compile()
	if err := fl.Run(ctx, flow.Opts{}); err != nil {
		state, err2 := f.GetInfrastructureState()
		return nil, state, errors.Join(err, err2)
	}
	status, err := f.GetInfrastructureStatus(ctx)
	state, err2 := f.GetInfrastructureState()
	err = errors.Join(err, err2)
	return status, state, err
}

func (f *FlowContext) buildReconcileGraph() *flow.Graph {
	g := flow.NewGraph("Azure infrastructure reconciliation")
	resourceGroup := f.AddTask(g, "ensure resource group", f.EnsureResourceGroup)
	vnet := f.AddTask(g, "ensure vnet", f.EnsureVirtualNetwork, shared.Dependencies(resourceGroup))
	f.AddTask(g, "ensure availability set", f.EnsureAvailabilitySet, shared.DoIf(f.adapter.AvailabilitySetConfig() != nil), shared.Dependencies(resourceGroup))
	routeTable := f.AddTask(g, "ensure route table", f.EnsureRouteTable, shared.Dependencies(resourceGroup))
	securityGroup := f.AddTask(g, "ensure security group", f.EnsureSecurityGroup, shared.Dependencies(resourceGroup))
	ip := f.AddTask(g, "ensure pips", f.EnsurePublicIPs, shared.Dependencies(resourceGroup))
	nat := f.AddTask(g, "ensure nats", f.EnsureNatGateways, shared.Dependencies(resourceGroup, ip))
	f.AddTask(g, "ensure subnets", f.EnsureSubnets, shared.Dependencies(vnet, routeTable, securityGroup, nat))
	return g
}

// Delete deletes all resources managed by the reconciler
func (f *FlowContext) Delete(ctx context.Context) error {
	// special case where the credentials were invalid from the beginning
	if k := f.whiteboard.Get(infrastructure.CreatedResourcesExistKey); k == nil {
		return nil
	}

	graph := flow.NewGraph("Azure infrastructure deletion")
	foreignSubnets := f.AddTask(graph, "delete subnets in foreign resource group", f.DeleteSubnetsInForeignGroup)
	f.AddTask(graph, "delete resource group", f.DeleteResourceGroup, shared.Dependencies(foreignSubnets))
	fl := graph.Compile()
	if err := fl.Run(ctx, flow.Opts{}); err != nil {
		return flow.Causes(err)
	}

	return nil
}

func (f *FlowContext) StatePersist() shared.FlowStatePersistor {
	return func(ctx context.Context, _ shared.FlatMap) error {
		state, err := f.GetInfrastructureState()
		if err != nil {
			return err
		}
		return f.persistFunc(state)
	}
}
