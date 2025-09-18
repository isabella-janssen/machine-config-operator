package common

import (
	"context"
	"flag"
	"sync"

	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	mcfgclientset "github.com/openshift/client-go/machineconfiguration/clientset/versioned"
	machineclientset "github.com/openshift/client-go/machine/clientset/versioned"
	"github.com/openshift/machine-config-operator/pkg/webhooks"
)

var (
	// Webhook server configuration flags
	webhookPort     = flag.Int("webhook-port", 9443, "Port for the admission webhook server")
	webhookCertPath = flag.String("webhook-cert-path", "/etc/certs/tls.crt", "Path to the webhook TLS certificate")
	webhookKeyPath  = flag.String("webhook-key-path", "/etc/certs/tls.key", "Path to the webhook TLS private key")
	enableWebhook   = flag.Bool("enable-mcp-deletion-webhook", true, "Enable the MCP deletion validation webhook")
)

// WebhookManager manages the admission webhooks for the machine config controller
type WebhookManager struct {
	webhookServer *webhooks.WebhookServer
	cancel        context.CancelFunc
	wg            sync.WaitGroup
}

// NewWebhookManager creates a new webhook manager
func NewWebhookManager(kubeClient kubernetes.Interface, mcfgClient mcfgclientset.Interface, machineClient machineclientset.Interface) (*WebhookManager, error) {
	if !*enableWebhook {
		klog.Info("MCP deletion webhook is disabled")
		return &WebhookManager{}, nil
	}

	webhookServer, err := webhooks.NewWebhookServer(
		*webhookPort,
		*webhookCertPath,
		*webhookKeyPath,
		kubeClient,
		mcfgClient,
		machineClient,
	)
	if err != nil {
		return nil, err
	}

	return &WebhookManager{
		webhookServer: webhookServer,
	}, nil
}

// Start starts the webhook server
func (wm *WebhookManager) Start(ctx context.Context) error {
	if wm.webhookServer == nil {
		klog.V(2).Info("Webhook server is disabled, skipping")
		return nil
	}

	webhookCtx, cancel := context.WithCancel(ctx)
	wm.cancel = cancel

	wm.wg.Add(1)
	go func() {
		defer wm.wg.Done()
		if err := wm.webhookServer.Start(webhookCtx); err != nil {
			klog.Errorf("Webhook server failed: %v", err)
		}
	}()

	klog.Info("Webhook manager started")
	return nil
}

// Stop stops the webhook server gracefully
func (wm *WebhookManager) Stop() {
	if wm.cancel != nil {
		wm.cancel()
	}
	wm.wg.Wait()
	klog.Info("Webhook manager stopped")
}
