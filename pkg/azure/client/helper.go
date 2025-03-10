// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/cloud"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/go-autorest/autorest"
	azerrors "github.com/AzureAD/microsoft-authentication-library-for-go/apps/errors"
	corev1 "k8s.io/api/core/v1"

	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	azuretypes "github.com/gardener/gardener-extension-provider-azure/pkg/azure"
)

// FilterNotFoundError returns nil for NotFound errors.
func FilterNotFoundError(err error) error {
	if err == nil {
		return nil
	}
	if IsAzureAPINotFoundError(err) {
		return nil
	}
	return err
}

func isAzureAPIStatusError(err error, status int) bool {
	switch e := err.(type) {
	case autorest.DetailedError: // error from old azure client
		if code, ok := e.StatusCode.(int); ok && code == status {
			return true
		}
		if e.Response != nil && e.Response.StatusCode == status {
			return true
		}
	case *azcore.ResponseError: // error from new azure SDK client
		if e.StatusCode == status {
			return true
		}
	}

	cerr := azerrors.CallErr{}
	if errors.As(err, &cerr) {
		return cerr.Resp != nil && cerr.Resp.StatusCode == status
	}

	return false
}

// IsAzureAPINotFoundError tries to determine if an error is a resource not found error.
func IsAzureAPINotFoundError(err error) bool {
	return isAzureAPIStatusError(err, http.StatusNotFound)
}

// IsAzureAPIUnauthorized tries to determine if the API error is due to unauthorized access
func IsAzureAPIUnauthorized(err error) bool {
	if isAzureAPIStatusError(err, http.StatusUnauthorized) {
		return true
	}

	inErr := &azidentity.AuthenticationFailedError{}
	return errors.As(err, &inErr)
}

func DefaultCloudConfiguration(cloudConfiguration *azure.CloudConfiguration) azure.CloudConfiguration {
	if cloudConfiguration == nil {
		return azure.CloudConfiguration{
			Name: azure.AzurePublicCloudName,
		}
	}
	return *cloudConfiguration
}

// AzureCloudConfigurationFromSecret returns a matching cloudConfiguration corresponding to a well known cloud instance for the given region
func AzureCloudConfigurationFromSecret(secret *corev1.Secret) (cloud.Configuration, error) {
	if v, ok := secret.Data[azuretypes.AzureCloud]; ok {
		return AzureCloudConfigurationFromCloudConfiguration(&azure.CloudConfiguration{Name: string(v)})
	}
	if v, ok := secret.Data[azuretypes.DNSAzureCloud]; ok {
		return AzureCloudConfigurationFromCloudConfiguration(&azure.CloudConfiguration{Name: string(v)})
	}
	return AzureCloudConfigurationFromCloudConfiguration(nil)
}

// AzureCloudConfigurationFromCloudConfiguration returns the cloud.Configuration corresponding to the given cloud configuration name (as part of our CloudConfiguration).
func AzureCloudConfigurationFromCloudConfiguration(cloudConfiguration *azure.CloudConfiguration) (cloud.Configuration, error) {
	cloudConfigurationName := DefaultCloudConfiguration(cloudConfiguration).Name
	switch {
	case strings.EqualFold(cloudConfigurationName, azure.AzurePublicCloudName):
		return cloud.AzurePublic, nil
	case strings.EqualFold(cloudConfigurationName, azure.AzureGovCloudName):
		return cloud.AzureGovernment, nil
	case strings.EqualFold(cloudConfigurationName, azure.AzureChinaCloudName):
		return cloud.AzureChina, nil

	default:
		return cloud.Configuration{}, fmt.Errorf("unknown cloud configuration name '%s'", cloudConfigurationName)
	}
}
