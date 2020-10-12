// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package worker

import (
	"context"

	api "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/helper"
	"github.com/gardener/gardener/extensions/pkg/controller/worker"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	"github.com/pkg/errors"
)

// UpdateMachineImagesStatus stores the used machine images for the `Worker` resource in the worker-provider-status.
func (w *workerDelegate) UpdateMachineImagesStatus(ctx context.Context) error {
	if w.machineImages == nil {
		if err := w.generateMachineConfig(ctx); err != nil {
			return err
		}
	}

	// Decode the current worker provider status.
	workerStatus, err := w.decodeWorkerProviderStatus()
	if err != nil {
		return err
	}

	workerStatus.MachineImages = w.machineImages
	return w.updateWorkerProviderStatus(ctx, workerStatus)
}

func (w *workerDelegate) findMachineImage(name, version string) (urn, id *string, acceleratedNetworking *bool, err error) {
	machineImage, err := helper.FindImageFromCloudProfile(w.cloudProfileConfig, name, version)
	if err == nil {
		return machineImage.URN, machineImage.ID, machineImage.AcceleratedNetworking, nil
	}

	// Try to look up machine image in worker provider status as it was not found in componentconfig.
	if providerStatus := w.worker.Status.ProviderStatus; providerStatus != nil {
		workerStatus := &api.WorkerStatus{}
		if _, _, err := w.Decoder().Decode(providerStatus.Raw, nil, workerStatus); err != nil {
			return nil, nil, nil, errors.Wrapf(err, "could not decode worker status of worker '%s'", kutil.ObjectName(w.worker))
		}

		machineImage, err := helper.FindMachineImage(workerStatus.MachineImages, name, version)
		if err != nil {
			return nil, nil, nil, worker.ErrorMachineImageNotFound(name, version)
		}

		return machineImage.URN, machineImage.ID, machineImage.AcceleratedNetworking, nil
	}

	return nil, nil, nil, worker.ErrorMachineImageNotFound(name, version)
}

func appendMachineImage(machineImages []api.MachineImage, machineImage api.MachineImage) []api.MachineImage {
	if _, err := helper.FindMachineImage(machineImages, machineImage.Name, machineImage.Version); err != nil {
		return append(machineImages, machineImage)
	}
	return machineImages
}
