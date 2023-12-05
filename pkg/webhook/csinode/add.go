//  Copyright (c) 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package csinode

import (
	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/go-logr/logr"
	v1 "k8s.io/api/admissionregistration/v1"
	storagev1 "k8s.io/api/storage/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

const (
	AzureDiskCSIName string = "disk.csi.azure.com"
)

var (
	WebhookName = "csinode-allocatable-webhook"
	logger      = log.Log.WithName(WebhookName)
	drivers     = []string{AzureDiskCSIName}
)

// AddToManager creates a new topology webhook.
func AddToManager(mgr manager.Manager) (*extensionswebhook.Webhook, error) {
	logger.Info("Adding webhook to manager")
	types := []extensionswebhook.Type{extensionswebhook.Type{
		Obj: &storagev1.CSINode{},
	}}

	handler, err := extensionswebhook.NewHandlerWithShootClient(mgr, types, NewMutator(mgr, logger, Args{
		Drivers: map[string]func(logr.Logger, *storagev1.CSINodeDriver, *extensionsv1alpha1.WorkerPool) error{
			AzureDiskCSIName: GenericCSINodeMutate,
		},
	}), logger)
	if err != nil {
		return nil, err
	}

	wh := &extensionswebhook.Webhook{
		Name:           WebhookName,
		Path:           WebhookName,
		Target:         extensionswebhook.TargetShoot,
		Types:          types,
		Webhook:        nil,
		Handler:        handler,
		Selector:       nil,
		ObjectSelector: nil,
		FailurePolicy:  toPtr(v1.Fail),
		TimeoutSeconds: nil,
	}

	return wh, nil
}

func toPtr[T any](t T) *T {
	return &t
}
