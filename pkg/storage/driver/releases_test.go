/*
Copyright 2016 The Kubernetes Authors All rights reserved.
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

package driver

import (
	"encoding/base64"
	"reflect"
	"testing"

	"github.com/gogo/protobuf/proto"
	"k8s.io/kubernetes/pkg/runtime"

	_ "k8s.io/helm/api/install"
	"k8s.io/helm/client/clientset/fake"
	rspb "k8s.io/helm/pkg/proto/hapi/release"
)

// newTestFixture initializes a FakeReleaseInterface.
// ConfigMaps are created for each release provided.
func newTestFixtureReleases(t *testing.T, releases ...*rspb.Release) *Releases {
	return NewReleases(fake.NewFakeExtensionClient(initFakeTPRs(t, releases...)...).Releases("default"))
}

// initFakeTPRs initializes the FakeReleaseInterface with the set of releases.
func initFakeTPRs(t *testing.T, releases ...*rspb.Release) []runtime.Object {
	var objects []runtime.Object
	for _, rls := range releases {
		objkey := testKey(rls.Name, rls.Version)

		r, err := newReleasesObject(objkey, rls, nil)
		if err != nil {
			t.Fatalf("Failed to create configmap: %s", err)
		}
		r.Namespace = "default"
		var obj runtime.Object = r
		objects = append(objects, obj)
	}
	return objects
}

func TestReleaseName(t *testing.T) {
	c := newTestFixtureReleases(t)
	if c.Name() != ReleasesDriverName {
		t.Errorf("Expected name to be %q, got %q", ReleasesDriverName, c.Name())
	}
}

func TestReleaseGet(t *testing.T) {
	vers := int32(1)
	name := "smug-pigeon"
	namespace := "default"
	key := testKey(name, vers)
	rel := releaseStub(name, vers, namespace, rspb.Status_DEPLOYED)

	releases := newTestFixtureReleases(t, []*rspb.Release{rel}...)

	// get release with key
	got, err := releases.Get(key)
	if err != nil {
		t.Fatalf("Failed to get release: %s", err)
	}
	// compare fetched release with original
	if !reflect.DeepEqual(rel, got) {
		t.Errorf("Expected {%q}, got {%q}", rel, got)
	}
}

func TestUNcompressedReleaseGet(t *testing.T) {
	vers := int32(1)
	name := "smug-pigeon"
	namespace := "default"
	key := testKey(name, vers)
	rel := releaseStub(name, vers, namespace, rspb.Status_DEPLOYED)

	// Create a test fixture which contains an uncompressed release
	r, err := newReleasesObject(key, rel, nil)
	if err != nil {
		t.Fatalf("Failed to create configmap: %s", err)
	}
	b, err := proto.Marshal(rel)
	if err != nil {
		t.Fatalf("Failed to marshal release: %s", err)
	}
	r.Spec.Data = base64.StdEncoding.EncodeToString(b)
	releases := NewReleases(fake.NewFakeExtensionClient(initFakeTPRs(t, rel)...).Releases("default"))

	// get release with key
	got, err := releases.Get(key)
	if err != nil {
		t.Fatalf("Failed to get release: %s", err)
	}
	// compare fetched release with original
	if !reflect.DeepEqual(rel, got) {
		t.Errorf("Expected {%q}, got {%q}", rel, got)
	}
}

func TestReleaseList(t *testing.T) {
	releases := newTestFixtureReleases(t, []*rspb.Release{
		releaseStub("key-1", 1, "default", rspb.Status_DELETED),
		releaseStub("key-2", 1, "default", rspb.Status_DELETED),
		releaseStub("key-3", 1, "default", rspb.Status_DEPLOYED),
		releaseStub("key-4", 1, "default", rspb.Status_DEPLOYED),
		releaseStub("key-5", 1, "default", rspb.Status_SUPERSEDED),
		releaseStub("key-6", 1, "default", rspb.Status_SUPERSEDED),
	}...)

	// list all deleted releases
	del, err := releases.List(func(rel *rspb.Release) bool {
		return rel.Info.Status.Code == rspb.Status_DELETED
	})
	// check
	if err != nil {
		t.Errorf("Failed to list deleted: %s", err)
	}
	if len(del) != 2 {
		t.Errorf("Expected 2 deleted, got %d:\n%v\n", len(del), del)
	}

	// list all deployed releases
	dpl, err := releases.List(func(rel *rspb.Release) bool {
		return rel.Info.Status.Code == rspb.Status_DEPLOYED
	})
	// check
	if err != nil {
		t.Errorf("Failed to list deployed: %s", err)
	}
	if len(dpl) != 2 {
		t.Errorf("Expected 2 deployed, got %d", len(dpl))
	}

	// list all superseded releases
	ssd, err := releases.List(func(rel *rspb.Release) bool {
		return rel.Info.Status.Code == rspb.Status_SUPERSEDED
	})
	// check
	if err != nil {
		t.Errorf("Failed to list superseded: %s", err)
	}
	if len(ssd) != 2 {
		t.Errorf("Expected 2 superseded, got %d", len(ssd))
	}
}

func TestReleaseCreate(t *testing.T) {
	releases := newTestFixtureReleases(t)

	vers := int32(1)
	name := "smug-pigeon"
	namespace := "default"
	key := testKey(name, vers)
	rel := releaseStub(name, vers, namespace, rspb.Status_DEPLOYED)

	// store the release in a configmap
	if err := releases.Create(key, rel); err != nil {
		t.Fatalf("Failed to create release with key %q: %s", key, err)
	}

	// get the release back
	got, err := releases.Get(key)
	if err != nil {
		t.Fatalf("Failed to get release with key %q: %s", key, err)
	}

	// compare created release with original
	if !reflect.DeepEqual(rel, got) {
		t.Errorf("Expected {%q}, got {%q}", rel, got)
	}
}

func TestReleaseUpdate(t *testing.T) {
	vers := int32(1)
	name := "smug-pigeon"
	namespace := "default"
	key := testKey(name, vers)
	rel := releaseStub(name, vers, namespace, rspb.Status_DEPLOYED)

	releases := newTestFixtureReleases(t, []*rspb.Release{rel}...)

	// modify release status code
	rel.Info.Status.Code = rspb.Status_SUPERSEDED

	// perform the update
	if err := releases.Update(key, rel); err != nil {
		t.Fatalf("Failed to update release: %s", err)
	}

	// fetch the updated release
	got, err := releases.Get(key)
	if err != nil {
		t.Fatalf("Failed to get release with key %q: %s", key, err)
	}

	// check release has actually been updated by comparing modified fields
	if rel.Info.Status.Code != got.Info.Status.Code {
		t.Errorf("Expected status %s, got status %s", rel.Info.Status.Code, got.Info.Status.Code)
	}
}
