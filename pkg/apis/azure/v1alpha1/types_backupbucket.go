// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// BackupBucketConfig is the provider-specific configuration for backup buckets/entries
type BackupBucketConfig struct {
	metav1.TypeMeta `json:",inline"`
	// CloudConfiguration contains config that controls which cloud to connect to.
	// +optional
	CloudConfiguration *CloudConfiguration `json:"cloudConfiguration,omitempty"`
	// CredentialRotation controls the behavior of the BackupBucket credential rotation.
	CredentialRotation *CredentialRotation `json:"credentialRotation,omitempty"`
}

// CredentialRotation controls the behavior of the BackupBucket credential rotation.
type CredentialRotation struct {
	// Enabled specifies if the credential rotation for backupbuckets should be enabled.
	Enabled bool `json:"enabled"`
	// RotatePeriod is the period after which the credential will not be used anymore. The actual rotation will happen
	RotatePeriod metav1.Duration `json:"rotatePeriod"`
}
