package engine

import (
	"fmt"
	"os"
	"path/filepath"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// KubeConfig holds connection parameters
type KubeConfig struct {
	Kubeconfig string
	Context    string
}

// NewKubeClient creates a Kubernetes client from config
func NewKubeClient(cfg KubeConfig) (*kubernetes.Clientset, *rest.Config, error) {
	var restConfig *rest.Config
	var err error

	// Try in-cluster config first (guardian mode)
	restConfig, err = rest.InClusterConfig()
	if err != nil {
		// Fall back to kubeconfig
		kubeconfigPath := cfg.Kubeconfig
		if kubeconfigPath == "" {
			kubeconfigPath = defaultKubeconfig()
		}

		loadingRules := &clientcmd.ClientConfigLoadingRules{
			ExplicitPath: kubeconfigPath,
		}

		overrides := &clientcmd.ConfigOverrides{}
		if cfg.Context != "" {
			overrides.CurrentContext = cfg.Context
		}

		kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, overrides)
		restConfig, err = kubeConfig.ClientConfig()
		if err != nil {
			return nil, nil, fmt.Errorf("failed to build kubeconfig: %w", err)
		}
	}

	// Increase QPS for scan operations
	restConfig.QPS = 100
	restConfig.Burst = 200

	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	return clientset, restConfig, nil
}

// GetClusterName returns the current cluster name
func GetClusterName(cfg KubeConfig) string {
	kubeconfigPath := cfg.Kubeconfig
	if kubeconfigPath == "" {
		kubeconfigPath = defaultKubeconfig()
	}

	loadingRules := &clientcmd.ClientConfigLoadingRules{
		ExplicitPath: kubeconfigPath,
	}
	overrides := &clientcmd.ConfigOverrides{}
	if cfg.Context != "" {
		overrides.CurrentContext = cfg.Context
	}

	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, overrides)
	rawConfig, err := kubeConfig.RawConfig()
	if err != nil {
		return "unknown"
	}

	ctx := rawConfig.CurrentContext
	if cfg.Context != "" {
		ctx = cfg.Context
	}

	if c, ok := rawConfig.Contexts[ctx]; ok {
		return c.Cluster
	}
	return ctx
}

func defaultKubeconfig() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".kube", "config")
}
