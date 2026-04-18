# shlink-ingress-controller

A Kubernetes controller that automatically creates and deletes [Shlink](https://shlink.io) short URLs for annotated Ingress resources.

## How it works

The controller watches all Ingress resources cluster-wide. When an Ingress is annotated with `shlink.vollminlab.com/slug`, the controller calls the Shlink REST API to create a short URL pointing to the Ingress host. When the annotation is removed or the Ingress is deleted, the short URL is cleaned up via a finalizer.

The long URL is derived from the first host in `spec.rules[0].host`.

## Annotation

| Annotation | Description |
|---|---|
| `shlink.vollminlab.com/slug` | Desired short URL slug (e.g. `my-app` → `go.vollminlab.com/my-app`) |

Example:

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: my-app
  annotations:
    shlink.vollminlab.com/slug: my-app
spec:
  rules:
    - host: my-app.vollminlab.com
      ...
```

This creates `https://go.vollminlab.com/my-app` → `https://my-app.vollminlab.com`.

The actual short links — e.g. `https://vollm.in/my-app` — also resolve to the same destination. Shlink is configured with `vollm.in` as an additional domain, so both `go.vollminlab.com/<slug>` and `vollm.in/<slug>` work. The controller only needs to call the API once; Shlink handles both domains automatically.

## Prerequisites

### Shlink API key Secret

The controller reads its API key from a Kubernetes Secret named `shlink-credentials` in the `shlink` namespace. The Secret must have a key `initial-api-key`:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: shlink-credentials
  namespace: shlink
type: Opaque
stringData:
  initial-api-key: <your-shlink-api-key>
```

Create the API key in the Shlink admin UI or via the Shlink CLI before deploying the controller.

## Deploying via Helm

The Helm chart is published to Harbor as an OCI artifact. Harbor is an internal container registry running in the Kubernetes cluster at `harbor.vollminlab.com` — see `k8s-vollminlab-cluster` for its deployment.

### Add the registry

```sh
helm registry login harbor.vollminlab.com
```

### Install

```sh
helm install shlink-ingress-controller \
  oci://harbor.vollminlab.com/vollminlab/charts/shlink-ingress-controller \
  --namespace shlink \
  --create-namespace
```

### Key values

| Value | Default | Description |
|---|---|---|
| `image.repository` | `harbor.vollminlab.com/vollminlab/shlink-ingress-controller` | Controller image |
| `image.tag` | _(chart appVersion)_ | Image tag |
| `shlink.apiUrl` | `https://go.vollminlab.com/rest/v3` | Shlink REST API base URL |
| `shlink.secretName` | `shlink-credentials` | Secret name containing the API key |
| `shlink.secretNamespace` | `shlink` | Namespace of the API key Secret |

Override values with `--set` or a values file as needed.

## RBAC

The chart creates:

- **ClusterRole / ClusterRoleBinding** — `get`, `list`, `watch`, `update` on Ingresses (cluster-wide)
- **Role / RoleBinding** — `get`, `list`, `watch` on Secrets in `shlink.secretNamespace`
