/*
Copyright The Helm Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package tiller

import (
	"bytes"
	"reflect"
	"testing"
	"text/template"

	"github.com/ghodss/yaml"

	"k8s.io/helm/pkg/chartutil"
	"k8s.io/helm/pkg/proto/hapi/release"
	util "k8s.io/helm/pkg/releaseutil"
)

func TestSortManifests(t *testing.T) {

	data := []struct {
		name     []string
		path     string
		kind     []string
		hooks    map[string][]release.Hook_Event
		manifest string
	}{
		{
			name:  []string{"first"},
			path:  "one",
			kind:  []string{"Job"},
			hooks: map[string][]release.Hook_Event{"first": {release.Hook_PRE_INSTALL}},
			manifest: `apiVersion: v1
kind: Job
metadata:
  name: first
  labels:
    doesnot: matter
  annotations:
    "helm.sh/hook": pre-install
`,
		},
		{
			name:  []string{"second"},
			path:  "two",
			kind:  []string{"ReplicaSet"},
			hooks: map[string][]release.Hook_Event{"second": {release.Hook_POST_INSTALL}},
			manifest: `kind: ReplicaSet
apiVersion: v1beta1
metadata:
  name: second
  annotations:
    "helm.sh/hook": post-install
`,
		}, {
			name:  []string{"third"},
			path:  "three",
			kind:  []string{"ReplicaSet"},
			hooks: map[string][]release.Hook_Event{"third": nil},
			manifest: `kind: ReplicaSet
apiVersion: v1beta1
metadata:
  name: third
  annotations:
    "helm.sh/hook": no-such-hook
`,
		}, {
			name:  []string{"fourth"},
			path:  "four",
			kind:  []string{"Pod"},
			hooks: map[string][]release.Hook_Event{"fourth": nil},
			manifest: `kind: Pod
apiVersion: v1
metadata:
  name: fourth
  annotations:
    nothing: here`,
		}, {
			name:  []string{"fifth"},
			path:  "five",
			kind:  []string{"ReplicaSet"},
			hooks: map[string][]release.Hook_Event{"fifth": {release.Hook_POST_DELETE, release.Hook_POST_INSTALL}},
			manifest: `kind: ReplicaSet
apiVersion: v1beta1
metadata:
  name: fifth
  annotations:
    "helm.sh/hook": post-delete, post-install
`,
		}, {
			// Regression test: files with an underscore in the base name should be skipped.
			name:     []string{"sixth"},
			path:     "six/_six",
			kind:     []string{"ReplicaSet"},
			hooks:    map[string][]release.Hook_Event{"sixth": nil},
			manifest: `invalid manifest`, // This will fail if partial is not skipped.
		}, {
			// Regression test: files with no content should be skipped.
			name:     []string{"seventh"},
			path:     "seven",
			kind:     []string{"ReplicaSet"},
			hooks:    map[string][]release.Hook_Event{"seventh": nil},
			manifest: "",
		},
		{
			name:  []string{"eighth", "example-test"},
			path:  "eight",
			kind:  []string{"ConfigMap", "Pod"},
			hooks: map[string][]release.Hook_Event{"eighth": nil, "example-test": {release.Hook_RELEASE_TEST_SUCCESS}},
			manifest: `kind: ConfigMap
apiVersion: v1
metadata:
  name: eighth
data:
  name: value
---
apiVersion: v1
kind: Pod
metadata:
  name: example-test
  annotations:
    "helm.sh/hook": test-success
`,
		},
	}

	manifests := make(map[string]string, len(data))
	for _, o := range data {
		manifests[o.path] = o.manifest
	}

	hs, generic, err := sortManifests(manifests, chartutil.NewVersionSet("v1", "v1beta1"), InstallOrder)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	// This test will fail if 'six' or 'seven' was added.
	if len(generic) != 2 {
		t.Errorf("Expected 2 generic manifests, got %d", len(generic))
	}

	if len(hs) != 4 {
		t.Errorf("Expected 4 hooks, got %d", len(hs))
	}

	for _, out := range hs {
		found := false
		for _, expect := range data {
			if out.Path == expect.path {
				found = true
				if out.Path != expect.path {
					t.Errorf("Expected path %s, got %s", expect.path, out.Path)
				}
				nameFound := false
				for _, expectedName := range expect.name {
					if out.Name == expectedName {
						nameFound = true
					}
				}
				if !nameFound {
					t.Errorf("Got unexpected name %s", out.Name)
				}
				kindFound := false
				for _, expectedKind := range expect.kind {
					if out.Kind == expectedKind {
						kindFound = true
					}
				}
				if !kindFound {
					t.Errorf("Got unexpected kind %s", out.Kind)
				}

				expectedHooks := expect.hooks[out.Name]
				if !reflect.DeepEqual(expectedHooks, out.Events) {
					t.Errorf("expected events: %v but got: %v", expectedHooks, out.Events)
				}

			}
		}
		if !found {
			t.Errorf("Result not found: %v", out)
		}
	}

	// Verify the sort order
	sorted := []Manifest{}
	for _, s := range data {
		manifests := util.SplitManifests(s.manifest)

		for _, m := range manifests {
			var sh util.SimpleHead
			err := yaml.Unmarshal([]byte(m), &sh)
			if err != nil {
				// This is expected for manifests that are corrupt or empty.
				t.Log(err)
				continue
			}

			name := sh.Metadata.Name

			//only keep track of non-hook manifests
			if err == nil && s.hooks[name] == nil {
				another := Manifest{
					Content: m,
					Name:    name,
					Head:    &sh,
				}
				sorted = append(sorted, another)
			}
		}
	}

	sorted = sortByKind(sorted, InstallOrder)
	for i, m := range generic {
		if m.Content != sorted[i].Content {
			t.Errorf("Expected %q, got %q", m.Content, sorted[i].Content)
		}
	}
}

var manifestTemplate = `
apiVersion: apiextensions.k8s.io/v1beta1
kind: CustomResourceDefinition
metadata:
  name: example.com
  labels:
    app: example-crd
  annotations:
    helm.sh/hook: crd-install
{{- if .HookDeletePolicy}}
    {{ .HookDeletePolicy }}
{{- end }}
{{- if .HookDeleteTimeout}}
    {{ .HookDeleteTimeout }}
{{- end }}
spec:
  group: example.com
  version: v1alpha1
  names:
    kind: example
    plural: examples
  scope: Cluster
`

type manifestTemplateData struct {
	HookDeletePolicy, HookDeleteTimeout string
}

func TestSortManifestsHookDeletion(t *testing.T) {
	testCases := map[string]struct {
		templateData    manifestTemplateData
		hasDeletePolicy bool
		deletePolicy    release.Hook_DeletePolicy
		deleteTimeout   int64
	}{
		"No delete policy": {
			templateData:    manifestTemplateData{},
			hasDeletePolicy: false,
			deletePolicy:    release.Hook_BEFORE_HOOK_CREATION,
			deleteTimeout:   0,
		},
		"Delete policy, no delete timeout": {
			templateData: manifestTemplateData{
				HookDeletePolicy: "helm.sh/hook-delete-policy: before-hook-creation",
			},
			hasDeletePolicy: true,
			deletePolicy:    release.Hook_BEFORE_HOOK_CREATION,
			deleteTimeout:   defaultHookDeleteTimeoutInSeconds,
		},
		"Delete policy and delete timeout": {
			templateData: manifestTemplateData{
				HookDeletePolicy:  "helm.sh/hook-delete-policy: hook-succeeded",
				HookDeleteTimeout: `helm.sh/hook-delete-timeout: "420"`,
			},
			hasDeletePolicy: true,
			deletePolicy:    release.Hook_SUCCEEDED,
			deleteTimeout:   420,
		},
	}

	for tn, tc := range testCases {
		t.Run(tn, func(t *testing.T) {
			tmpl := template.Must(template.New("manifest").Parse(manifestTemplate))
			var buf bytes.Buffer
			err := tmpl.Execute(&buf, tc.templateData)
			if err != nil {
				t.Error(err)
			}

			manifests := map[string]string{
				"exampleManifest": buf.String(),
			}

			hs, _, err := sortManifests(manifests, chartutil.NewVersionSet("v1", "v1beta1"), InstallOrder)
			if err != nil {
				t.Error(err)
			}

			if got, want := len(hs), 1; got != want {
				t.Errorf("expected %d hooks, but got %d", want, got)
			}
			hook := hs[0]

			if len(hook.DeletePolicies) == 0 {
				if tc.hasDeletePolicy {
					t.Errorf("expected a policy, but got zero")
				}
			} else {
				if !tc.hasDeletePolicy {
					t.Errorf("expected no delete policies, but got one")
				}
				policy := hook.DeletePolicies[0]
				if got, want := policy, tc.deletePolicy; got != want {
					t.Errorf("expected delete policy %q, but got %q", want, got)
				}
			}

			if got, want := hook.DeleteTimeout, tc.deleteTimeout; got != want {
				t.Errorf("expected timeout %d, but got %d", want, got)
			}
		})
	}
}

func TestVersionSet(t *testing.T) {
	vs := chartutil.NewVersionSet("v1", "v1beta1", "extensions/alpha5", "batch/v1")

	if l := len(vs); l != 4 {
		t.Errorf("Expected 4, got %d", l)
	}

	if !vs.Has("extensions/alpha5") {
		t.Error("No match for alpha5")
	}

	if vs.Has("nosuch/extension") {
		t.Error("Found nonexistent extension")
	}
}
