# Helm Chart & GitOps Deployment Design

**Date:** 2026-04-12
**Scope:** Package the shlink-ingress-controller as a Helm chart, publish it to Harbor OCI, and wire it into k8s-vollminlab-cluster via Flux.

## Problem

The controller has source code and a Docker image but no deployment artifacts. Deployment manifests (RBAC, Deployment, ServiceAccount) belong in the GitOps repo, consistent with every other workload in the cluster.

## Approach

Helm chart in this repo, pushed to Harbor as an OCI artifact by CI, consumed by Flux via `OCIRepository` + `HelmRelease` in k8s-vollminlab-cluster. Matches existing cluster patterns; Renovate handles version bumps automatically.

## Helm Chart (`charts/shlink-ingress-controller/`)

### Files

| File | Purpose |
|------|---------|
| `Chart.yaml` | name `shlink-ingress-controller`, version + appVersion from git tag |
| `values.yaml` | image repo/tag, shlink API URL, secret name/namespace, resource limits |
| `templates/deployment.yaml` | Single-replica Deployment; shlink flags passed as CLI args |
| `templates/serviceaccount.yaml` | ServiceAccount for the controller pod |
| `templates/clusterrole.yaml` | `get/list/watch/update` on `ingresses` and `ingresses/finalizers` cluster-wide |
| `templates/clusterrolebinding.yaml` | Binds ClusterRole to ServiceAccount |
| `templates/role.yaml` | `get` on `secrets` scoped to deployment namespace only |
| `templates/rolebinding.yaml` | Binds Role in deployment namespace |

### Key values

```yaml
image:
  repository: harbor.vollminlab.com/vollminlab/shlink-ingress-controller
  tag: ""  # defaults to chart appVersion

shlink:
  apiUrl: https://go.vollminlab.com/rest/v3
  secretName: shlink-credentials
  secretNamespace: shlink

resources: {}
```

### RBAC rationale

- **ClusterRole** for Ingresses: the controller watches all namespaces so any annotated Ingress across the cluster creates a short URL.
- **Role** (namespace-scoped) for Secrets: the API key secret lives in the `shlink` namespace only; scoping Secret access to that namespace is tighter than a ClusterRole.

## CI Changes (`.github/workflows/build-push.yml`)

Add a `publish-chart` job that runs after `build` succeeds on tag pushes:

1. `helm registry login harbor.vollminlab.com` — uses existing `HARBOR_USERNAME` / `HARBOR_PASSWORD` secrets
2. `helm package charts/shlink-ingress-controller --version <semver> --app-version <semver>` — strips leading `v` from tag (e.g. `v0.1.0` → `0.1.0`)
3. `helm push shlink-ingress-controller-*.tgz oci://harbor.vollminlab.com/vollminlab`

Chart version and Docker image tag are always in lockstep (same git tag drives both).

## GitOps Changes (`k8s-vollminlab-cluster`)

### New files

**`flux-system/repositories/shlink-ingress-controller-helmrepository.yaml`**
`OCIRepository` pointing at `oci://harbor.vollminlab.com/vollminlab/shlink-ingress-controller`, tag pinned to initial release.

**`clusters/vollminlab-cluster/shlink/shlink-ingress-controller/app/helmrelease.yaml`**
`HelmRelease` with `chartRef` to the OCIRepository and `valuesFrom` the ConfigMap.

**`clusters/vollminlab-cluster/shlink/shlink-ingress-controller/app/configmap.yaml`**
Helm values: image repo, shlink API URL, secret name `shlink-credentials`, secret namespace `shlink`.

**`clusters/vollminlab-cluster/shlink/shlink-ingress-controller/app/kustomization.yaml`**
Lists helmrelease.yaml and configmap.yaml.

### Modified files

| File | Change |
|------|--------|
| `flux-system/repositories/kustomization.yaml` | Add `- shlink-ingress-controller-helmrepository.yaml` |
| `clusters/vollminlab-cluster/shlink/kustomization.yaml` | Add `- shlink-ingress-controller/app` |

### No new SealedSecret needed

The existing `shlink-credentials` SealedSecret in the `shlink` namespace already has `initial-api-key`. The controller references it directly.

### No new Flux Kustomization CR needed

The existing shlink namespace Kustomization CR in `flux-system/flux-kustomizations/` already reconciles everything under `clusters/vollminlab-cluster/shlink/`.
