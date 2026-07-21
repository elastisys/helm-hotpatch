# Helm Hotpatch

This Helm post-renderer plugin allows for patching the rendered Helm chart templates.

## Quickstart

Create a patch.

```sh
mkdir ./patches
cat > ./patches/patch.yaml <<EOF
target: testchart/templates/service.yaml
patches:
  - action: patch
    data:
      apiVersion: v1
      kind: Service
      metadata:
        name: test
      spec:
        type: LoadBalancer
EOF
```

Template without patching.

```console
$ helm template foo integration/testdata/chart -s templates/service.yaml
---
# Source: testchart/templates/service.yaml
apiVersion: v1
kind: Service
metadata:
  name: test
spec:
  ports:
  - name: http
    port: 80
    protocol: TCP
    targetPort: http
  type: ClusterIP
```

Template with patching.

```console
$ helm template foo integration/testdata/chart -s templates/service.yaml --post-renderer hotpatch
---
# Source: testchart/templates/service.yaml
apiVersion: v1
kind: Service
metadata:
  name: test
spec:
  ports:
  - name: http
    port: 80
    protocol: TCP
    targetPort: http
  type: LoadBalancer
```

## Patching

```yaml
# Each patch targets a single template.
target: chart/templates/name.yaml
# There are three different patch actions, add, remove, patch.
patches:
  # Patches with action add introduces a new manifest to the rendered output.
  - action: add
    data:
      apiVersion: v1
      kind: Service
      metadata:
        name: new-temporary-service
      spec:
        ...
  # Patches with action remove deletes the manifest that matches.
  - action: remove
    data:
      apiVersion: v1
      kind: Service
      metadata:
        name: existing-service
  # Patches with action patch changes the manifest that matches.
  - action: patch
    data:
      apiVersion: v1
      kind: Service
      metadata:
        name: existing-service-2
      ports:
        - port: 8080
          targetPort: 80
```

## Release

To create a new release, update the version in [plugin.yaml](/plugin.yaml) and push the commit to the main branch.
