//go:build integration

package controller_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// waitFor polls condition every 100ms until it returns true or 10s elapses.
func waitFor(t *testing.T, condition func() bool) bool {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
	return false
}

func hasFinalizer(ingress *networkingv1.Ingress) bool {
	for _, f := range ingress.Finalizers {
		if f == "shlink.vollminlab.com/finalizer" {
			return true
		}
	}
	return false
}

// TestIntegration_CreateIngress_CreatesShortURL verifies that when an Ingress
// with the slug annotation is created, the controller fires and calls CreateShortURL.
func TestIntegration_CreateIngress_CreatesShortURL(t *testing.T) {
	ctx := context.Background()
	mockShlink.Reset(nil) // no slugs pre-exist in Shlink

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "int-create"}}
	require.NoError(t, k8sClient.Create(ctx, ns))
	t.Cleanup(func() { _ = k8sClient.Delete(ctx, ns) })

	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "myapp",
			Namespace:   "int-create",
			Annotations: map[string]string{"shlink.vollminlab.com/slug": "myapp"},
		},
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{{Host: "myapp.vollminlab.com"}},
		},
	}
	require.NoError(t, k8sClient.Create(ctx, ingress))

	// Finalizer appearing proves the controller reconciled.
	ok := waitFor(t, func() bool {
		updated := &networkingv1.Ingress{}
		_ = k8sClient.Get(ctx, types.NamespacedName{Name: "myapp", Namespace: "int-create"}, updated)
		return hasFinalizer(updated)
	})
	require.True(t, ok, "finalizer never appeared — controller did not reconcile")

	calls := mockShlink.CreateCalls()
	require.Len(t, calls, 1)
	assert.Equal(t, "myapp", calls[0].slug)
	assert.Equal(t, "https://myapp.vollminlab.com", calls[0].longURL)
}

// TestIntegration_ExistingSlug_DoesNotCallCreate verifies that when the Shlink
// API already has the slug, the controller skips creating it.
func TestIntegration_ExistingSlug_DoesNotCallCreate(t *testing.T) {
	ctx := context.Background()
	mockShlink.Reset(map[string]string{"exists": "https://exists.vollminlab.com"}) // slug pre-exists in Shlink

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "int-exists"}}
	require.NoError(t, k8sClient.Create(ctx, ns))
	t.Cleanup(func() { _ = k8sClient.Delete(ctx, ns) })

	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "exists",
			Namespace:   "int-exists",
			Annotations: map[string]string{"shlink.vollminlab.com/slug": "exists"},
		},
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{{Host: "exists.vollminlab.com"}},
		},
	}
	require.NoError(t, k8sClient.Create(ctx, ingress))

	ok := waitFor(t, func() bool {
		updated := &networkingv1.Ingress{}
		_ = k8sClient.Get(ctx, types.NamespacedName{Name: "exists", Namespace: "int-exists"}, updated)
		return hasFinalizer(updated)
	})
	require.True(t, ok, "finalizer never appeared — controller did not reconcile")

	assert.Empty(t, mockShlink.CreateCalls(), "CreateShortURL should not be called when slug already exists")
}

// TestIntegration_DeleteIngress_DeletesShortURL verifies that deleting an Ingress
// causes the controller to call DeleteShortURL and remove the finalizer.
func TestIntegration_DeleteIngress_DeletesShortURL(t *testing.T) {
	ctx := context.Background()
	mockShlink.Reset(nil)

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "int-delete"}}
	require.NoError(t, k8sClient.Create(ctx, ns))
	t.Cleanup(func() { _ = k8sClient.Delete(ctx, ns) })

	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "todelete",
			Namespace:   "int-delete",
			Annotations: map[string]string{"shlink.vollminlab.com/slug": "todelete"},
		},
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{{Host: "todelete.vollminlab.com"}},
		},
	}
	require.NoError(t, k8sClient.Create(ctx, ingress))

	// Wait for finalizer so we know the controller has processed the create.
	ok := waitFor(t, func() bool {
		updated := &networkingv1.Ingress{}
		_ = k8sClient.Get(ctx, types.NamespacedName{Name: "todelete", Namespace: "int-delete"}, updated)
		return hasFinalizer(updated)
	})
	require.True(t, ok, "finalizer never appeared after create")

	// Now delete the Ingress.
	require.NoError(t, k8sClient.Delete(ctx, ingress))

	// Wait for the Ingress to be fully gone (finalizer removed + GC'd).
	ok = waitFor(t, func() bool {
		err := k8sClient.Get(ctx, types.NamespacedName{Name: "todelete", Namespace: "int-delete"}, &networkingv1.Ingress{})
		return client.IgnoreNotFound(err) == nil && err != nil // NotFound
	})
	require.True(t, ok, "Ingress was never deleted — finalizer may not have been removed")

	assert.Equal(t, []string{"todelete"}, mockShlink.DeleteCalls())
}

// TestIntegration_MissingAPIKeySecret_ReturnsError verifies that if the controller
// cannot find the API key Secret, reconciliation fails gracefully (requeue without crash).
// We observe this indirectly: the finalizer never appears within the timeout.
func TestIntegration_MissingAPIKeySecret_ReturnsError(t *testing.T) {
	// The integration suite wires the controller with ShlinkClientOverride, so the
	// Secret lookup path is bypassed here. That path is covered by unit tests.
	t.Skip("API key Secret lookup is covered by unit tests; integration suite always uses ShlinkClientOverride")
}

// TestIntegration_NoAnnotation_DoesNotReconcile verifies that an Ingress without
// the slug annotation is ignored by the controller.
func TestIntegration_NoAnnotation_DoesNotReconcile(t *testing.T) {
	ctx := context.Background()
	mockShlink.Reset(nil)

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "int-noanno"}}
	require.NoError(t, k8sClient.Create(ctx, ns))
	t.Cleanup(func() { _ = k8sClient.Delete(ctx, ns) })

	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "noanno",
			Namespace: "int-noanno",
			// no slug annotation
		},
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{{Host: "noanno.vollminlab.com"}},
		},
	}
	require.NoError(t, k8sClient.Create(ctx, ingress))

	// Give the controller time to process it (it will run, but should do nothing).
	time.Sleep(500 * time.Millisecond)

	updated := &networkingv1.Ingress{}
	require.NoError(t, k8sClient.Get(ctx, types.NamespacedName{Name: "noanno", Namespace: "int-noanno"}, updated))
	assert.False(t, hasFinalizer(updated), "finalizer should not be added to an Ingress without the slug annotation")
	assert.Empty(t, mockShlink.CreateCalls())
}
