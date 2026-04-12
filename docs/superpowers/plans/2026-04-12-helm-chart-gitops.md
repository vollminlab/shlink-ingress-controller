# Helm Chart & GitOps Deployment Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Package the shlink-ingress-controller as a Helm chart, publish it to Harbor OCI via CI, and wire it into k8s-vollminlab-cluster via Flux.

**Architecture:** A `charts/shlink-ingress-controller/` Helm chart lives in this repo and is pushed to `harbor.vollminlab.com/vollminlab/shlink-ingress-controller` as an OCI artifact by CI on each version tag. The gitops repo references it via an `OCIRepository` + `HelmRelease` under the existing `shlink` namespace, reusing the existing `shlink-credentials` secret for the API key.

**Tech Stack:** Helm 3, GitHub Actions, Flux CD (OCIRepository v1beta2 + HelmRelease v2), Kubernetes RBAC, Harbor OCI registry.

**Repo split:** Tasks 1–9 are in `shlink-ingress-controller`. Tasks 10–14 are in `k8s-vollminlab-cluster`. Tasks 10–14 must be done after the first git tag is pushed and CI has published the chart.

---

## File Map

### `shlink-ingress-controller` repo (new files)
- `charts/shlink-ingress-controller/Chart.yaml` — chart metadata
- `charts/shlink-ingress-controller/values.yaml` — default values
- `charts/shlink-ingress-controller/templates/_helpers.tpl` — name/label helpers
- `charts/shlink-ingress-controller/templates/serviceaccount.yaml`
- `charts/shlink-ingress-controller/templates/clusterrole.yaml` — ingress get/list/watch/update cluster-wide
- `charts/shlink-ingress-controller/templates/clusterrolebinding.yaml`
- `charts/shlink-ingress-controller/templates/role.yaml` — secrets get, scoped to shlink namespace
- `charts/shlink-ingress-controller/templates/rolebinding.yaml`
- `charts/shlink-ingress-controller/templates/deployment.yaml`

### `shlink-ingress-controller` repo (modified)
- `.github/workflows/build-push.yml` — add `publish-chart` job

### `k8s-vollminlab-cluster` repo (new files)
- `clusters/vollminlab-cluster/flux-system/repositories/shlink-ingress-controller-helmrepository.yaml` — OCIRepository
- `clusters/vollminlab-cluster/shlink/shlink-ingress-controller/app/helmrelease.yaml`
- `clusters/vollminlab-cluster/shlink/shlink-ingress-controller/app/configmap.yaml`
- `clusters/vollminlab-cluster/shlink/shlink-ingress-controller/app/kustomization.yaml`

### `k8s-vollminlab-cluster` repo (modified)
- `clusters/vollminlab-cluster/flux-system/repositories/kustomization.yaml` — add OCIRepository entry
- `clusters/vollminlab-cluster/shlink/kustomization.yaml` — add ingress-controller app

---

## Task 1: Chart scaffold

**Files:**
- Create: `charts/shlink-ingress-controller/Chart.yaml`
- Create: `charts/shlink-ingress-controller/values.yaml`

- [ ] **Step 1: Create Chart.yaml**

```yaml
# charts/shlink-ingress-controller/Chart.yaml
apiVersion: v2
name: shlink-ingress-controller
description: Kubernetes controller that creates Shlink short URLs for annotated Ingress resources
type: application
version: 0.1.0
appVersion: "0.1.0"
```

- [ ] **Step 2: Create values.yaml**

```yaml
# charts/shlink-ingress-controller/values.yaml
image:
  repository: harbor.vollminlab.com/vollminlab/shlink-ingress-controller
  tag: ""
  pullPolicy: IfNotPresent

shlink:
  apiUrl: https://go.vollminlab.com/rest/v3
  secretName: shlink-credentials
  secretNamespace: shlink

resources: {}

nameOverride: ""
fullnameOverride: ""
```

- [ ] **Step 3: Verify helm lint passes on the bare scaffold**

```bash
helm lint charts/shlink-ingress-controller
```

Expected: `1 chart(s) linted, 0 chart(s) failed`

- [ ] **Step 4: Commit**

```bash
git add charts/shlink-ingress-controller/Chart.yaml charts/shlink-ingress-controller/values.yaml
git commit -m "feat: add helm chart scaffold"
```

---

## Task 2: Helpers template

**Files:**
- Create: `charts/shlink-ingress-controller/templates/_helpers.tpl`

- [ ] **Step 1: Create _helpers.tpl**

```
{{- define "shlink-ingress-controller.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "shlink-ingress-controller.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{- define "shlink-ingress-controller.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "shlink-ingress-controller.labels" -}}
helm.sh/chart: {{ include "shlink-ingress-controller.chart" . }}
{{ include "shlink-ingress-controller.selectorLabels" . }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app: {{ include "shlink-ingress-controller.name" . }}
env: production
category: apps
{{- end }}

{{- define "shlink-ingress-controller.selectorLabels" -}}
app.kubernetes.io/name: {{ include "shlink-ingress-controller.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{- define "shlink-ingress-controller.serviceAccountName" -}}
{{ include "shlink-ingress-controller.fullname" . }}
{{- end }}
```

- [ ] **Step 2: Commit**

```bash
git add charts/shlink-ingress-controller/templates/_helpers.tpl
git commit -m "feat: add helm chart helpers"
```

---

## Task 3: ServiceAccount template

**Files:**
- Create: `charts/shlink-ingress-controller/templates/serviceaccount.yaml`

- [ ] **Step 1: Create serviceaccount.yaml**

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: {{ include "shlink-ingress-controller.serviceAccountName" . }}
  namespace: {{ .Release.Namespace }}
  labels:
    {{- include "shlink-ingress-controller.labels" . | nindent 4 }}
```

- [ ] **Step 2: Verify it renders correctly**

```bash
helm template shlink-ingress-controller charts/shlink-ingress-controller -n shlink | grep -A5 "kind: ServiceAccount"
```

Expected output contains:
```
kind: ServiceAccount
metadata:
  name: shlink-ingress-controller
  namespace: shlink
```

- [ ] **Step 3: Commit**

```bash
git add charts/shlink-ingress-controller/templates/serviceaccount.yaml
git commit -m "feat: add serviceaccount helm template"
```

---

## Task 4: ClusterRole and ClusterRoleBinding templates

**Files:**
- Create: `charts/shlink-ingress-controller/templates/clusterrole.yaml`
- Create: `charts/shlink-ingress-controller/templates/clusterrolebinding.yaml`

The controller calls `r.Update(ctx, &ingress)` to patch finalizers, which requires `update` on `ingresses` (not a subresource). It also uses `list` and `watch` via the manager cache.

- [ ] **Step 1: Create clusterrole.yaml**

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: {{ include "shlink-ingress-controller.fullname" . }}
  labels:
    {{- include "shlink-ingress-controller.labels" . | nindent 4 }}
rules:
  - apiGroups: ["networking.k8s.io"]
    resources: ["ingresses"]
    verbs: ["get", "list", "watch", "update"]
```

- [ ] **Step 2: Create clusterrolebinding.yaml**

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: {{ include "shlink-ingress-controller.fullname" . }}
  labels:
    {{- include "shlink-ingress-controller.labels" . | nindent 4 }}
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: {{ include "shlink-ingress-controller.fullname" . }}
subjects:
  - kind: ServiceAccount
    name: {{ include "shlink-ingress-controller.serviceAccountName" . }}
    namespace: {{ .Release.Namespace }}
```

- [ ] **Step 3: Verify both render**

```bash
helm template shlink-ingress-controller charts/shlink-ingress-controller -n shlink | grep "kind: Cluster"
```

Expected:
```
kind: ClusterRole
kind: ClusterRoleBinding
```

- [ ] **Step 4: Commit**

```bash
git add charts/shlink-ingress-controller/templates/clusterrole.yaml charts/shlink-ingress-controller/templates/clusterrolebinding.yaml
git commit -m "feat: add clusterrole helm templates"
```

---

## Task 5: Role and RoleBinding templates

**Files:**
- Create: `charts/shlink-ingress-controller/templates/role.yaml`
- Create: `charts/shlink-ingress-controller/templates/rolebinding.yaml`

These are namespace-scoped to `shlink` (where the API key Secret lives), keeping Secret access tighter than a ClusterRole would allow.

- [ ] **Step 1: Create role.yaml**

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: {{ include "shlink-ingress-controller.fullname" . }}
  namespace: {{ .Values.shlink.secretNamespace }}
  labels:
    {{- include "shlink-ingress-controller.labels" . | nindent 4 }}
rules:
  - apiGroups: [""]
    resources: ["secrets"]
    verbs: ["get"]
```

- [ ] **Step 2: Create rolebinding.yaml**

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: {{ include "shlink-ingress-controller.fullname" . }}
  namespace: {{ .Values.shlink.secretNamespace }}
  labels:
    {{- include "shlink-ingress-controller.labels" . | nindent 4 }}
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: {{ include "shlink-ingress-controller.fullname" . }}
subjects:
  - kind: ServiceAccount
    name: {{ include "shlink-ingress-controller.serviceAccountName" . }}
    namespace: {{ .Release.Namespace }}
```

- [ ] **Step 3: Verify both render**

```bash
helm template shlink-ingress-controller charts/shlink-ingress-controller -n shlink | grep "kind: Role"
```

Expected:
```
kind: Role
kind: RoleBinding
```

- [ ] **Step 4: Commit**

```bash
git add charts/shlink-ingress-controller/templates/role.yaml charts/shlink-ingress-controller/templates/rolebinding.yaml
git commit -m "feat: add role helm templates for secret access"
```

---

## Task 6: Deployment template

**Files:**
- Create: `charts/shlink-ingress-controller/templates/deployment.yaml`

The controller binary accepts three flags matching the Helm values: `--shlink-api-url`, `--shlink-secret-name`, `--shlink-secret-namespace`.

- [ ] **Step 1: Create deployment.yaml**

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ include "shlink-ingress-controller.fullname" . }}
  namespace: {{ .Release.Namespace }}
  labels:
    {{- include "shlink-ingress-controller.labels" . | nindent 4 }}
spec:
  replicas: 1
  selector:
    matchLabels:
      {{- include "shlink-ingress-controller.selectorLabels" . | nindent 6 }}
  template:
    metadata:
      labels:
        {{- include "shlink-ingress-controller.labels" . | nindent 8 }}
    spec:
      serviceAccountName: {{ include "shlink-ingress-controller.serviceAccountName" . }}
      containers:
        - name: controller
          image: "{{ .Values.image.repository }}:{{ .Values.image.tag | default .Chart.AppVersion }}"
          imagePullPolicy: {{ .Values.image.pullPolicy }}
          args:
            - --shlink-api-url={{ .Values.shlink.apiUrl }}
            - --shlink-secret-name={{ .Values.shlink.secretName }}
            - --shlink-secret-namespace={{ .Values.shlink.secretNamespace }}
          resources:
            {{- toYaml .Values.resources | nindent 12 }}
```

- [ ] **Step 2: Verify the image tag defaults to appVersion**

```bash
helm template shlink-ingress-controller charts/shlink-ingress-controller -n shlink | grep "image:"
```

Expected:
```
image: harbor.vollminlab.com/vollminlab/shlink-ingress-controller:0.1.0
```

- [ ] **Step 3: Commit**

```bash
git add charts/shlink-ingress-controller/templates/deployment.yaml
git commit -m "feat: add deployment helm template"
```

---

## Task 7: Full chart validation

**Files:** none (validation only)

- [ ] **Step 1: Run helm lint**

```bash
helm lint charts/shlink-ingress-controller
```

Expected: `1 chart(s) linted, 0 chart(s) failed`

- [ ] **Step 2: Render full template and check all kinds are present**

```bash
helm template shlink-ingress-controller charts/shlink-ingress-controller -n shlink | grep "^kind:"
```

Expected (order may vary):
```
kind: ServiceAccount
kind: ClusterRole
kind: ClusterRoleBinding
kind: Role
kind: RoleBinding
kind: Deployment
```

- [ ] **Step 3: Dry-run against cluster (requires kubeconfig)**

```bash
helm template shlink-ingress-controller charts/shlink-ingress-controller -n shlink \
  | kubectl apply --dry-run=client -f -
```

Expected: all resources print `(dry run)` with no errors.

---

## Task 8: Add publish-chart CI job

**Files:**
- Modify: `.github/workflows/build-push.yml`

- [ ] **Step 1: Add the publish-chart job**

Open `.github/workflows/build-push.yml` and append this job after the existing `build` job:

```yaml
  publish-chart:
    runs-on: self-hosted
    needs: build
    steps:
      - uses: actions/checkout@v4

      - name: Set up Helm
        uses: azure/setup-helm@v4

      - name: Log in to Harbor OCI registry
        run: |
          helm registry login harbor.vollminlab.com \
            --username ${{ secrets.HARBOR_USERNAME }} \
            --password ${{ secrets.HARBOR_PASSWORD }}

      - name: Package and push chart
        run: |
          VERSION="${{ github.ref_name }}"
          SEMVER="${VERSION#v}"
          helm package charts/shlink-ingress-controller \
            --version "${SEMVER}" \
            --app-version "${SEMVER}"
          helm push "shlink-ingress-controller-${SEMVER}.tgz" \
            oci://harbor.vollminlab.com/vollminlab
```

- [ ] **Step 2: Commit**

```bash
git add .github/workflows/build-push.yml
git commit -m "ci: publish helm chart to Harbor OCI on tag push"
```

---

## Task 9: Tag and push to trigger CI

**Files:** none

This publishes the Docker image and Helm chart to Harbor. The gitops tasks (10–14) require the chart to exist in Harbor before Flux can pull it.

- [ ] **Step 1: Tag the release**

```bash
git tag v0.1.0
git push origin main --tags
```

- [ ] **Step 2: Verify CI completes**

Monitor the Actions run in GitHub. Both `build` and `publish-chart` jobs must succeed.

- [ ] **Step 3: Confirm chart exists in Harbor**

```bash
helm show chart oci://harbor.vollminlab.com/vollminlab/shlink-ingress-controller --version 0.1.0
```

Expected: prints the Chart.yaml content including `version: 0.1.0`.

---

## Task 10: OCIRepository in gitops repo

**Working directory for tasks 10–14:** `k8s-vollminlab-cluster`

Follow the git workflow in `.claude/rules/git-workflow.md` — create a branch before making changes.

**Files:**
- Create: `clusters/vollminlab-cluster/flux-system/repositories/shlink-ingress-controller-helmrepository.yaml`
- Modify: `clusters/vollminlab-cluster/flux-system/repositories/kustomization.yaml`

- [ ] **Step 1: Create the OCIRepository**

```yaml
# clusters/vollminlab-cluster/flux-system/repositories/shlink-ingress-controller-helmrepository.yaml
apiVersion: source.toolkit.fluxcd.io/v1beta2
kind: OCIRepository
metadata:
  name: shlink-ingress-controller-repo
  namespace: flux-system
  labels:
    app: shlink-ingress-controller
    env: production
    category: apps
spec:
  interval: 1h
  url: oci://harbor.vollminlab.com/vollminlab/shlink-ingress-controller
  ref:
    tag: "0.1.0"
  layerSelector:
    mediaType: "application/vnd.cncf.helm.chart.content.v1.tar+gzip"
    operation: copy
```

- [ ] **Step 2: Register it in the repositories index**

In `clusters/vollminlab-cluster/flux-system/repositories/kustomization.yaml`, add to the `resources` list (alphabetical order, after `shlink-helmrepository.yaml`):

```yaml
  - shlink-ingress-controller-helmrepository.yaml
```

- [ ] **Step 3: Commit**

```bash
git add clusters/vollminlab-cluster/flux-system/repositories/shlink-ingress-controller-helmrepository.yaml \
        clusters/vollminlab-cluster/flux-system/repositories/kustomization.yaml
git commit -m "feat: add OCIRepository for shlink-ingress-controller"
```

---

## Task 11: HelmRelease and ConfigMap

**Files:**
- Create: `clusters/vollminlab-cluster/shlink/shlink-ingress-controller/app/helmrelease.yaml`
- Create: `clusters/vollminlab-cluster/shlink/shlink-ingress-controller/app/configmap.yaml`
- Create: `clusters/vollminlab-cluster/shlink/shlink-ingress-controller/app/kustomization.yaml`

- [ ] **Step 1: Create helmrelease.yaml**

```yaml
apiVersion: helm.toolkit.fluxcd.io/v2
kind: HelmRelease
metadata:
  name: shlink-ingress-controller
  namespace: shlink
  labels:
    app: shlink-ingress-controller
    env: production
    category: apps
spec:
  interval: 10m
  chartRef:
    kind: OCIRepository
    name: shlink-ingress-controller-repo
    namespace: flux-system
  valuesFrom:
    - kind: ConfigMap
      name: shlink-ingress-controller-values
```

- [ ] **Step 2: Create configmap.yaml**

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: shlink-ingress-controller-values
  namespace: shlink
  labels:
    app: shlink-ingress-controller
    env: production
    category: apps
data:
  values.yaml: |
    image:
      repository: harbor.vollminlab.com/vollminlab/shlink-ingress-controller
      tag: "0.1.0"
    shlink:
      apiUrl: https://go.vollminlab.com/rest/v3
      secretName: shlink-credentials
      secretNamespace: shlink
```

- [ ] **Step 3: Create kustomization.yaml**

```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
  - helmrelease.yaml
  - configmap.yaml
```

- [ ] **Step 4: Commit**

```bash
git add clusters/vollminlab-cluster/shlink/shlink-ingress-controller/
git commit -m "feat: add shlink-ingress-controller HelmRelease and values"
```

---

## Task 12: Wire into shlink namespace kustomization

**Files:**
- Modify: `clusters/vollminlab-cluster/shlink/kustomization.yaml`

- [ ] **Step 1: Add the new app to the namespace kustomization**

In `clusters/vollminlab-cluster/shlink/kustomization.yaml`, add to `resources`:

```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
  - namespace.yaml
  - shlink/app
  - shlink-web/app
  - shlink-db/app
  - shlink-ingress-controller/app
```

- [ ] **Step 2: Commit**

```bash
git add clusters/vollminlab-cluster/shlink/kustomization.yaml
git commit -m "feat: register shlink-ingress-controller in shlink namespace kustomization"
```

---

## Task 13: Open PR and verify Flux reconciles

**Files:** none

- [ ] **Step 1: Push branch and open PR**

Follow `.claude/rules/git-workflow.md`. Push the branch and open a PR against `main`.

- [ ] **Step 2: After PR merges, verify Flux picks up the OCIRepository**

```bash
flux get sources oci -n flux-system | grep shlink-ingress-controller
```

Expected: status `True`, reason `Succeeded`.

- [ ] **Step 3: Verify the HelmRelease deploys**

```bash
flux get helmrelease shlink-ingress-controller -n shlink
```

Expected: status `True`, reason `InstallSucceeded` or `UpgradeSucceeded`.

- [ ] **Step 4: Confirm the controller pod is running**

```bash
kubectl get pods -n shlink -l app.kubernetes.io/name=shlink-ingress-controller
```

Expected: one pod in `Running` state.

- [ ] **Step 5: Smoke test — annotate an Ingress and verify the short URL is created**

```bash
kubectl annotate ingress <any-ingress> -n <namespace> shlink.vollminlab.com/slug=test-slug
```

Then check `https://go.vollminlab.com/test-slug` resolves to the ingress host.
