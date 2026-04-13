//go:build integration

package controller_test

import (
	"context"
	"os"
	"sync"
	"testing"

	networkingv1 "k8s.io/api/networking/v1"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"github.com/vollminlab/shlink-ingress-controller/internal/controller"
	"github.com/vollminlab/shlink-ingress-controller/internal/shlink"
)

var (
	k8sClient  client.Client
	testEnv    *envtest.Environment
	cancelFunc context.CancelFunc
	mockShlink *syncMockClient
)

func TestMain(m *testing.M) {
	scheme := k8sruntime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = networkingv1.AddToScheme(scheme)

	testEnv = &envtest.Environment{}
	cfg, err := testEnv.Start()
	if err != nil {
		panic("failed to start envtest: " + err.Error())
	}

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme})
	if err != nil {
		panic("failed to create k8s client: " + err.Error())
	}

	mockShlink = &syncMockClient{}

	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme:         scheme,
		LeaderElection: false,
		Metrics:        metricsserver.Options{BindAddress: "0"},
	})
	if err != nil {
		panic("failed to create manager: " + err.Error())
	}

	if err = (&controller.IngressReconciler{
		Client:                      mgr.GetClient(),
		Scheme:                      mgr.GetScheme(),
		ShlinkClientOverride:        mockShlink,
		ShlinkAPIKeySecretName:      "shlink-credentials",
		ShlinkAPIKeySecretNamespace: "shlink",
	}).SetupWithManager(mgr); err != nil {
		panic("failed to setup controller: " + err.Error())
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancelFunc = cancel
	go func() {
		if err := mgr.Start(ctx); err != nil && ctx.Err() == nil {
			panic("manager exited unexpectedly: " + err.Error())
		}
	}()

	code := m.Run()

	cancel()
	_ = testEnv.Stop()
	os.Exit(code)
}

// syncMockClient is a thread-safe, stateful Shlink client mock for integration tests.
// It mirrors real Shlink behavior: CreateShortURL makes the slug visible to GetShortURL,
// and DeleteShortURL removes it. This prevents spurious double-create calls when the
// controller reconciles twice (once on create, once after adding the finalizer).
type syncMockClient struct {
	mu          sync.Mutex
	urls        map[string]string // slug → longURL, simulates Shlink state
	createCalls []struct{ slug, longURL string }
	deleteCalls []string
}

// Reset prepares the mock for a new test. existingSlugs pre-populates the Shlink
// state (slug → longURL) so GetShortURL returns them as already existing.
func (m *syncMockClient) Reset(existingSlugs map[string]string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.urls = make(map[string]string)
	for k, v := range existingSlugs {
		m.urls[k] = v
	}
	m.createCalls = nil
	m.deleteCalls = nil
}

func (m *syncMockClient) GetShortURL(slug string) (*shlink.ShortURL, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if longURL, ok := m.urls[slug]; ok {
		return &shlink.ShortURL{ShortCode: slug, LongURL: longURL}, nil
	}
	return nil, nil
}

func (m *syncMockClient) CreateShortURL(slug, longURL string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.createCalls = append(m.createCalls, struct{ slug, longURL string }{slug, longURL})
	if m.urls == nil {
		m.urls = make(map[string]string)
	}
	m.urls[slug] = longURL
	return nil
}

func (m *syncMockClient) DeleteShortURL(slug string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.deleteCalls = append(m.deleteCalls, slug)
	delete(m.urls, slug)
	return nil
}

func (m *syncMockClient) CreateCalls() []struct{ slug, longURL string } {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]struct{ slug, longURL string }, len(m.createCalls))
	copy(out, m.createCalls)
	return out
}

func (m *syncMockClient) DeleteCalls() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]string, len(m.deleteCalls))
	copy(out, m.deleteCalls)
	return out
}
