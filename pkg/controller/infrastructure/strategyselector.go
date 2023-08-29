package infrastructure

import (
	"context"
	"fmt"

	"github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/terraformer"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/extensions"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/v1alpha1"
	"github.com/gardener/gardener-extension-provider-azure/pkg/internal"
	infrainternal "github.com/gardener/gardener-extension-provider-azure/pkg/internal/infrastructure"
)

// Reconciler is an interface for the infrastructure reconciliation.
type Reconciler interface {
	// Reconcile manages infrastructure resources according to spec.It returns the provider's InfrastructureStatus, a State object to persist and an error.
	Reconcile(ctx context.Context, infra *extensionsv1alpha1.Infrastructure, cluster *controller.Cluster) (*v1alpha1.InfrastructureStatus, *runtime.RawExtension, error)
	// Delete removes any created infrastructure resource on the provider.
	Delete(ctx context.Context, infra *extensionsv1alpha1.Infrastructure, cluster *controller.Cluster) error
}

// ReconcilerFactory can construct the different infrastructure reconciler implementations.
type ReconcilerFactory interface {
	Build(useFlow bool) (Reconciler, error)
}

// ReconcilerFactoryImpl is an implementation of a ReconcilerFactory
type ReconcilerFactoryImpl struct {
	ctx              context.Context
	log              logr.Logger
	a                *actuator
	infra            *extensionsv1alpha1.Infrastructure
	stateInitializer terraformer.StateConfigMapInitializer
}

// Build builds the Reconciler according to the arguments.
func (f ReconcilerFactoryImpl) Build(useFlow bool) (Reconciler, error) {
	tf, err := internal.NewTerraformerWithAuth(f.log, f.a.RESTConfig(), infrainternal.TerraformerPurpose, f.infra, f.a.disableProjectedTokenMount)
	if err != nil {
		return nil, err
	}

	if useFlow {
		reconciler, err := NewFlowReconciler(f.a, f.log, tf)
		if err != nil {
			return nil, fmt.Errorf("failed to init flow reconciler: %w", err)
		}
		return reconciler, nil
	}

	reconciler, err := NewTerraformReconciler(f.a, f.log, tf, f.stateInitializer)
	if err != nil {
		return nil, fmt.Errorf("failed to init terraform reconciler: %w", err)
	}
	return reconciler, nil
}

// StrategySelector decides the reconciler used.
type StrategySelector interface {
	Select(infrastructure *extensionsv1alpha1.Infrastructure, cluster *extensions.Cluster) (bool, error)
}

// SelectorFunc decides the reconciler used.
type SelectorFunc func(*extensionsv1alpha1.Infrastructure, *extensions.Cluster) (bool, error)

// Select selects the reconciler implementation.
func (s SelectorFunc) Select(infrastructure *extensionsv1alpha1.Infrastructure, cluster *extensions.Cluster) (bool, error) {
	return s(infrastructure, cluster)
}

// OnReconcile returns true if the operation should use the Flow for the given cluster.
func OnReconcile(infra *extensionsv1alpha1.Infrastructure, cluster *extensions.Cluster) (bool, error) {
	hasState, err := hasFlowState(infra.Status)
	if err != nil {
		return false, err
	}
	return hasState || HasFlowAnnotation(infra, cluster), nil
}

// OnDelete returns true if the operation should use the Flow deletion for the given cluster.
func OnDelete(infra *extensionsv1alpha1.Infrastructure, _ *extensions.Cluster) (bool, error) {
	return hasFlowState(infra.Status)
}

// OnRestore decides the reconciler used on migration.
var OnRestore = OnDelete
