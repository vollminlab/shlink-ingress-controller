package controller

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/vollminlab/shlink-ingress-controller/internal/shlink"
)

const (
	annotationSlug   = "shlink.vollminlab.com/slug"
	finalizer        = "shlink.vollminlab.com/finalizer"
	maxDeleteRetries = 3
)

// IngressReconciler watches Ingress resources and manages Shlink short URLs.
type IngressReconciler struct {
	client.Client
	Scheme                      *runtime.Scheme
	ShlinkBaseURL               string
	ShlinkAPIKeySecretName      string
	ShlinkAPIKeySecretNamespace string
	ShlinkClientOverride        shlink.Client // nil in production, set in tests
}

func (r *IngressReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var ingress networkingv1.Ingress
	if err := r.Get(ctx, req.NamespacedName, &ingress); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	slug, hasAnnotation := ingress.Annotations[annotationSlug]
	hasFinalizer := controllerutil.ContainsFinalizer(&ingress, finalizer)

	if !ingress.DeletionTimestamp.IsZero() {
		if hasFinalizer {
			r.deleteWithRetry(ctx, slug)
			controllerutil.RemoveFinalizer(&ingress, finalizer)
			if err := r.Update(ctx, &ingress); err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	if !hasAnnotation {
		return ctrl.Result{}, nil
	}

	if len(ingress.Spec.Rules) == 0 || ingress.Spec.Rules[0].Host == "" {
		logger.Info("ingress has no host, skipping", "ingress", req.NamespacedName)
		return ctrl.Result{}, nil
	}
	longURL := fmt.Sprintf("https://%s", ingress.Spec.Rules[0].Host)

	sc, err := r.shlinkClient(ctx)
	if err != nil {
		return ctrl.Result{}, err
	}

	existing, err := sc.GetShortURL(slug)
	if err != nil {
		return ctrl.Result{}, err
	}

	if existing == nil {
		if err := sc.CreateShortURL(slug, longURL); err != nil {
			return ctrl.Result{}, err
		}
		logger.Info("created short URL", "slug", slug, "longURL", longURL)
	} else {
		logger.Info("short URL already exists, skipping", "slug", slug)
	}

	if !controllerutil.ContainsFinalizer(&ingress, finalizer) {
		controllerutil.AddFinalizer(&ingress, finalizer)
		if err := r.Update(ctx, &ingress); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// deleteWithRetry attempts to delete the short link up to maxDeleteRetries times.
// It always returns nil — a Shlink outage must not permanently block Ingress deletion.
func (r *IngressReconciler) deleteWithRetry(ctx context.Context, slug string) {
	logger := log.FromContext(ctx)
	sc, err := r.shlinkClient(ctx)
	if err != nil {
		logger.Error(err, "could not build Shlink client for deletion, skipping", "slug", slug)
		return
	}
	for i := 0; i < maxDeleteRetries; i++ {
		if err := sc.DeleteShortURL(slug); err == nil {
			return
		}
	}
	logger.Error(err, "failed to delete short URL after retries, removing finalizer anyway", "slug", slug)
}

// shlinkClient returns the Shlink client to use. In tests ShlinkClientOverride takes precedence.
func (r *IngressReconciler) shlinkClient(ctx context.Context) (shlink.Client, error) {
	if r.ShlinkClientOverride != nil {
		return r.ShlinkClientOverride, nil
	}
	apiKey, err := r.getAPIKey(ctx)
	if err != nil {
		return nil, err
	}
	return shlink.New(r.ShlinkBaseURL, apiKey), nil
}

func (r *IngressReconciler) getAPIKey(ctx context.Context) (string, error) {
	var secret corev1.Secret
	if err := r.Get(ctx, types.NamespacedName{
		Name:      r.ShlinkAPIKeySecretName,
		Namespace: r.ShlinkAPIKeySecretNamespace,
	}, &secret); err != nil {
		return "", fmt.Errorf("getting Shlink API key secret: %w", err)
	}
	key := string(secret.Data["initial-api-key"])
	if key == "" {
		return "", fmt.Errorf("initial-api-key is empty in secret %s/%s", r.ShlinkAPIKeySecretNamespace, r.ShlinkAPIKeySecretName)
	}
	return key, nil
}

func (r *IngressReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&networkingv1.Ingress{}).
		Complete(r)
}
