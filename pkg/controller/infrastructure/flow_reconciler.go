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

package infrastructure

import (
	"context"

	"github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/terraformer"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/v1alpha1"
	"github.com/gardener/gardener-extension-provider-azure/pkg/controller/infrastructure/infraflow"
	"github.com/gardener/gardener-extension-provider-azure/pkg/internal/infrastructure"
)

// FlowReconciler an implementation of an infrastructure reconciler using native SDKs.
type FlowReconciler struct {
	client     client.Client
	restConfig *rest.Config
	log        logr.Logger
	tf         terraformer.Terraformer
}

// NewFlowReconciler creates a new flow reconciler.
func NewFlowReconciler(a *actuator, log logr.Logger, tf terraformer.Terraformer) (Reconciler, error) {
	return &FlowReconciler{
		client:     a.Client(),
		restConfig: a.RESTConfig(),
		log:        log,
		tf:         tf,
	}, nil
}

// Reconcile reconciles the infrastructure and returns the status (state of the world), the state (input for the next loops) and any errors that occurred.
func (f *FlowReconciler) Reconcile(ctx context.Context, infra *extensionsv1alpha1.Infrastructure, cluster *controller.Cluster) (*v1alpha1.InfrastructureStatus, *runtime.RawExtension, error) {
	if err := CleanupTerraformerResources(ctx, f.tf); err != nil {
		return nil, nil, err
	}

	factory, err := NewAzureClientFactory(ctx, f.client, infra.Spec.SecretRef)
	if err != nil {
		return nil, nil, err
	}

	fctx, err := infraflow.NewFlowContext(factory, f.log, infra, cluster)
	if err != nil {
		return nil, nil, err
	}

	status, state, err := fctx.Reconcile(ctx)
	if err != nil {
		return nil, nil, err
	}
	infraState, err := (&infrastructure.InfrastructureState{
		SavedProviderStatus: &runtime.RawExtension{
			Object: status,
		},
		FlowState: state,
	}).ToRawExtension()

	return status, infraState, err
}

// Delete deletes the infrastructure resource using the flow reconciler.
func (f *FlowReconciler) Delete(ctx context.Context, infra *extensionsv1alpha1.Infrastructure, cluster *controller.Cluster) error {
	factory, err := NewAzureClientFactory(ctx, f.client, infra.Spec.SecretRef)
	if err != nil {
		return err
	}

	fctx, err := infraflow.NewFlowContext(factory, f.log, infra, cluster)
	if err != nil {
		return err
	}

	return fctx.Delete(ctx)
}
