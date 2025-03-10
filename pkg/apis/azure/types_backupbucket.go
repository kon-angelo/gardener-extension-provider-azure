// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package azure

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// BackupBucketConfig is the provider-specific configuration for backup buckets/entries
type BackupBucketConfig struct {
	metav1.TypeMeta
	// CloudConfiguration contains config that controls which cloud to connect to.
	CloudConfiguration *CloudConfiguration
	// CredentialRotation controls the behavior of the BackupBucket credential rotation.
	CredentialRotation *CredentialRotation
}

// CredentialRotation controls the behavior of the BackupBucket credential rotation.
type CredentialRotation struct {
	Enabled      bool
	RotatePeriod metav1.Duration
}
