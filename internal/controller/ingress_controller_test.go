package controller_test

import (
    "context"
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    corev1 "k8s.io/api/core/v1"
    networkingv1 "k8s.io/api/networking/v1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/apimachinery/pkg/runtime"
    "k8s.io/apimachinery/pkg/types"
    apierrors "k8s.io/apimachinery/pkg/api/errors"
    clientgoscheme "k8s.io/client-go/kubernetes/scheme"
    ctrl "sigs.k8s.io/controller-runtime"
    "sigs.k8s.io/controller-runtime/pkg/client/fake"

    "github.com/vollminlab/shlink-ingress-controller/internal/controller"
    "github.com/vollminlab/shlink-ingress-controller/internal/shlink"
)

// mockShlinkClient records calls for test assertions.
type mockShlinkClient struct {
    getResult   *shlink.ShortURL
    getErr      error
    createErr   error
    deleteErr   error
    createCalls []struct{ slug, longURL string }
    deleteCalls []string
}

func (m *mockShlinkClient) GetShortURL(slug string) (*shlink.ShortURL, error) {
    return m.getResult, m.getErr
}
func (m *mockShlinkClient) CreateShortURL(slug, longURL string) error {
    m.createCalls = append(m.createCalls, struct{ slug, longURL string }{slug, longURL})
    return m.createErr
}
func (m *mockShlinkClient) DeleteShortURL(slug string) error {
    m.deleteCalls = append(m.deleteCalls, slug)
    return m.deleteErr
}

func newScheme() *runtime.Scheme {
    s := runtime.NewScheme()
    _ = clientgoscheme.AddToScheme(s)
    _ = networkingv1.AddToScheme(s)
    return s
}

func apiKeySecret() *corev1.Secret {
    return &corev1.Secret{
        ObjectMeta: metav1.ObjectMeta{Name: "shlink-credentials", Namespace: "shlink"},
        Data:       map[string][]byte{"initial-api-key": []byte("test-key")},
    }
}

func ingressWithSlug(name, namespace, host, slug string) *networkingv1.Ingress {
    return &networkingv1.Ingress{
        ObjectMeta: metav1.ObjectMeta{
            Name:        name,
            Namespace:   namespace,
            Annotations: map[string]string{"shlink.vollminlab.com/slug": slug},
        },
        Spec: networkingv1.IngressSpec{
            Rules: []networkingv1.IngressRule{{Host: host}},
        },
    }
}

func TestReconcile_NoAnnotation_Skips(t *testing.T) {
    ingress := &networkingv1.Ingress{
        ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
        Spec:       networkingv1.IngressSpec{Rules: []networkingv1.IngressRule{{Host: "test.vollminlab.com"}}},
    }
    mock := &mockShlinkClient{}
    r := &controller.IngressReconciler{
        Client:                      fake.NewClientBuilder().WithScheme(newScheme()).WithObjects(ingress).Build(),
        ShlinkAPIKeySecretName:      "shlink-credentials",
        ShlinkAPIKeySecretNamespace: "shlink",
        ShlinkClientOverride:        mock,
    }

    _, err := r.Reconcile(context.Background(), ctrl.Request{
        NamespacedName: types.NamespacedName{Name: "test", Namespace: "default"},
    })
    require.NoError(t, err)
    assert.Empty(t, mock.createCalls)
}

func TestReconcile_NewSlug_CreatesShortURL(t *testing.T) {
    ingress := ingressWithSlug("radarr", "mediastack", "radarr.vollminlab.com", "radarr")
    mock := &mockShlinkClient{getResult: nil} // slug does not exist
    r := &controller.IngressReconciler{
        Client:                      fake.NewClientBuilder().WithScheme(newScheme()).WithObjects(ingress, apiKeySecret()).Build(),
        ShlinkAPIKeySecretName:      "shlink-credentials",
        ShlinkAPIKeySecretNamespace: "shlink",
        ShlinkClientOverride:        mock,
    }

    _, err := r.Reconcile(context.Background(), ctrl.Request{
        NamespacedName: types.NamespacedName{Name: "radarr", Namespace: "mediastack"},
    })
    require.NoError(t, err)
    require.Len(t, mock.createCalls, 1)
    assert.Equal(t, "radarr", mock.createCalls[0].slug)
    assert.Equal(t, "https://radarr.vollminlab.com", mock.createCalls[0].longURL)

    // finalizer must be added
    updated := &networkingv1.Ingress{}
    require.NoError(t, r.Client.Get(context.Background(), types.NamespacedName{Name: "radarr", Namespace: "mediastack"}, updated))
    assert.Contains(t, updated.Finalizers, "shlink.vollminlab.com/finalizer")
}

func TestReconcile_ExistingSlug_Skips(t *testing.T) {
    ingress := ingressWithSlug("radarr", "mediastack", "radarr.vollminlab.com", "radarr")
    mock := &mockShlinkClient{getResult: &shlink.ShortURL{ShortCode: "radarr"}} // slug exists
    r := &controller.IngressReconciler{
        Client:                      fake.NewClientBuilder().WithScheme(newScheme()).WithObjects(ingress, apiKeySecret()).Build(),
        ShlinkAPIKeySecretName:      "shlink-credentials",
        ShlinkAPIKeySecretNamespace: "shlink",
        ShlinkClientOverride:        mock,
    }

    _, err := r.Reconcile(context.Background(), ctrl.Request{
        NamespacedName: types.NamespacedName{Name: "radarr", Namespace: "mediastack"},
    })
    require.NoError(t, err)
    assert.Empty(t, mock.createCalls) // must NOT create
}

func TestReconcile_Delete_DeletesShortURL(t *testing.T) {
    ingress := ingressWithSlug("radarr", "mediastack", "radarr.vollminlab.com", "radarr")
    // Add finalizer and deletion timestamp to simulate deletion in progress
    now := metav1.Now()
    ingress.DeletionTimestamp = &now
    ingress.Finalizers = []string{"shlink.vollminlab.com/finalizer"}

    mock := &mockShlinkClient{}
    r := &controller.IngressReconciler{
        Client:                      fake.NewClientBuilder().WithScheme(newScheme()).WithObjects(ingress, apiKeySecret()).Build(),
        ShlinkAPIKeySecretName:      "shlink-credentials",
        ShlinkAPIKeySecretNamespace: "shlink",
        ShlinkClientOverride:        mock,
    }

    _, err := r.Reconcile(context.Background(), ctrl.Request{
        NamespacedName: types.NamespacedName{Name: "radarr", Namespace: "mediastack"},
    })
    require.NoError(t, err)
    assert.Equal(t, []string{"radarr"}, mock.deleteCalls)

    // finalizer must be removed — in controller-runtime v0.23+ the fake client auto-deletes
    // objects when the last finalizer is removed from an object with a DeletionTimestamp,
    // so "not found" is also a valid proof that the finalizer was removed.
    updated := &networkingv1.Ingress{}
    getErr := r.Client.Get(context.Background(), types.NamespacedName{Name: "radarr", Namespace: "mediastack"}, updated)
    if getErr == nil {
        assert.NotContains(t, updated.Finalizers, "shlink.vollminlab.com/finalizer")
    } else {
        require.True(t, apierrors.IsNotFound(getErr), "expected NotFound, got: %v", getErr)
    }
}

func TestReconcile_NoHost_Skips(t *testing.T) {
    ingress := &networkingv1.Ingress{
        ObjectMeta: metav1.ObjectMeta{
            Name:        "nohost",
            Namespace:   "default",
            Annotations: map[string]string{"shlink.vollminlab.com/slug": "nohost"},
        },
        Spec: networkingv1.IngressSpec{Rules: []networkingv1.IngressRule{{Host: ""}}},
    }
    mock := &mockShlinkClient{}
    r := &controller.IngressReconciler{
        Client:                      fake.NewClientBuilder().WithScheme(newScheme()).WithObjects(ingress).Build(),
        ShlinkAPIKeySecretName:      "shlink-credentials",
        ShlinkAPIKeySecretNamespace: "shlink",
        ShlinkClientOverride:        mock,
    }

    _, err := r.Reconcile(context.Background(), ctrl.Request{
        NamespacedName: types.NamespacedName{Name: "nohost", Namespace: "default"},
    })
    require.NoError(t, err)
    assert.Empty(t, mock.createCalls)
}
