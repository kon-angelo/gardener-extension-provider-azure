package infrastructure

import (
	"context"
	"fmt"

	"github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/terraformer"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/helper"
	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/v1alpha1"
	azureclient "github.com/gardener/gardener-extension-provider-azure/pkg/azure/client"
	"github.com/gardener/gardener-extension-provider-azure/pkg/internal"
	"github.com/gardener/gardener-extension-provider-azure/pkg/internal/infrastructure"
)

// NewTerraformReconciler creates a new TerraformReconciler
func NewTerraformReconciler(a *actuator, logger logr.Logger, tf terraformer.Terraformer, stateInitializer terraformer.StateConfigMapInitializer) (Reconciler, error) {
	return &TerraformReconciler{
		Client:           a.client,
		Logger:           logger,
		StateInitializer: stateInitializer,
		Terraformer:      tf,
	}, nil
}

var _ Reconciler = &TerraformReconciler{}

// TerraformReconciler can reconcile infrastructure objects using Terraform.
type TerraformReconciler struct {
	Client           client.Client
	Logger           logr.Logger
	StateInitializer terraformer.StateConfigMapInitializer
	Terraformer      terraformer.Terraformer
}

// Reconcile reconciles the infrastructure resource according to spec.
func (r *TerraformReconciler) Reconcile(ctx context.Context, infra *extensionsv1alpha1.Infrastructure, cluster *controller.Cluster) (*v1alpha1.InfrastructureStatus, *runtime.RawExtension, error) {
	cfg, err := helper.InfrastructureConfigFromInfrastructure(infra)
	if err != nil {
		return nil, nil, err
	}
	terraformFiles, err := infrastructure.RenderTerraformerTemplate(infra, cfg, cluster)
	if err != nil {
		return nil, nil, err
	}

	if err := r.Terraformer.
		InitializeWith(ctx, terraformer.DefaultInitializer(r.Client, terraformFiles.Main, terraformFiles.Variables, terraformFiles.TFVars, r.StateInitializer)).
		Apply(ctx); err != nil {

		return nil, nil, fmt.Errorf("failed to apply the terraform config: %w", err)
	}

	status, err := infrastructure.ComputeTerraformStatus(ctx, r.Terraformer, infra, cfg, cluster)
	if err != nil {
		return nil, nil, err
	}
	state, err := r.getState(ctx, status)
	if err != nil {
		return nil, nil, err
	}

	return status, state, nil
}

// getState calculates the State resource after each reconciliation.
func (r *TerraformReconciler) getState(ctx context.Context, status *v1alpha1.InfrastructureStatus) (*runtime.RawExtension, error) {
	terraformState, err := r.Terraformer.GetRawState(ctx)
	if err != nil {
		return nil, err
	}

	stateByte, err := terraformState.Marshal()
	if err != nil {
		return nil, err
	}

	infraState := &infrastructure.InfrastructureState{
		SavedProviderStatus: &runtime.RawExtension{
			Object: status,
		},
		TerraformState: &runtime.RawExtension{
			Raw: stateByte,
		},
	}
	return infraState.ToRawExtension()
}

// Delete removes any created infrastructure resource on the provider.
func (r *TerraformReconciler) Delete(ctx context.Context, infra *extensionsv1alpha1.Infrastructure, cluster *controller.Cluster) error {
	var (
		tf = r.Terraformer
	)

	// terraform pod from previous reconciliation might still be running, ensure they are gone before doing any operations
	if err := tf.EnsureCleanedUp(ctx); err != nil {
		return err
	}

	azureClientFactory, err := NewAzureClientFactory(ctx, r.Client, infra.Spec.SecretRef)
	if err != nil {
		return err
	}

	cfg, err := helper.InfrastructureConfigFromInfrastructure(infra)
	if err != nil {
		return err
	}

	resourceGroupExists, err := infrastructure.IsShootResourceGroupAvailable(ctx, azureClientFactory, infra, cfg)
	if err != nil {
		if azureclient.IsAzureAPIUnauthorized(err) {
			r.Logger.Error(err, "Failed to check resource group availability due to invalid credentials")
		} else {
			return err
		}
	}

	if !resourceGroupExists {
		if !azureclient.IsAzureAPIUnauthorized(err) {
			if err := infrastructure.DeleteNodeSubnetIfExists(ctx, azureClientFactory, infra, cfg); err != nil {
				return err
			}
		}

		if err := tf.RemoveTerraformerFinalizerFromConfig(ctx); err != nil {
			return err
		}

		return tf.CleanupConfiguration(ctx)
	}

	// If the Terraform state is empty then we can exit early as we didn't create anything. Though, we clean up potentially
	// created configmaps/secrets related to the Terraformer.
	stateIsEmpty := tf.IsStateEmpty(ctx)
	if stateIsEmpty {
		r.Logger.Info("exiting early as infrastructure state is empty - nothing to do")
		return tf.CleanupConfiguration(ctx)
	}

	terraformFiles, err := infrastructure.RenderTerraformerTemplate(infra, cfg, cluster)
	if err != nil {
		return err
	}

	return tf.
		InitializeWith(ctx, terraformer.DefaultInitializer(r.Client, terraformFiles.Main, terraformFiles.Variables, terraformFiles.TFVars, r.StateInitializer)).
		SetEnvVars(internal.TerraformerEnvVars(infra.Spec.SecretRef)...).
		Destroy(ctx)
}
