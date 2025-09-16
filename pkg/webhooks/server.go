package webhooks

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	mcfgclientset "github.com/openshift/client-go/machineconfiguration/clientset/versioned"
	machineclientset "github.com/openshift/client-go/machine/clientset/versioned"
)

// WebhookServer manages the admission webhook server
type WebhookServer struct {
	server        *http.Server
	mcpValidation *MCPDeletionWebhook
}

// NewWebhookServer creates a new webhook server
func NewWebhookServer(port int, certPath, keyPath string, kubeClient kubernetes.Interface, mcfgClient mcfgclientset.Interface, machineClient machineclientset.Interface) (*WebhookServer, error) {
	// Load TLS certificates
	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load key pair: %v", err)
	}

	// Create the MCP deletion webhook
	mcpValidation := NewMCPDeletionWebhook(kubeClient, mcfgClient, machineClient)

	// Create HTTP server
	mux := http.NewServeMux()
	mux.Handle("/validate-mcp-deletion", mcpValidation)
	mux.HandleFunc("/healthz", healthzHandler)

	server := &http.Server{
		Addr:      fmt.Sprintf(":%d", port),
		TLSConfig: &tls.Config{Certificates: []tls.Certificate{cert}},
		Handler:   mux,
	}

	return &WebhookServer{
		server:        server,
		mcpValidation: mcpValidation,
	}, nil
}

// Start starts the webhook server
func (ws *WebhookServer) Start(ctx context.Context) error {
	klog.Infof("Starting webhook server on %s", ws.server.Addr)

	// Start the server in a goroutine
	go func() {
		if err := ws.server.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
			klog.Errorf("Failed to start webhook server: %v", err)
		}
	}()

	// Wait for context cancellation
	<-ctx.Done()
	
	// Graceful shutdown
	klog.Info("Shutting down webhook server...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	return ws.server.Shutdown(shutdownCtx)
}

// healthzHandler provides a health check endpoint
func healthzHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}
