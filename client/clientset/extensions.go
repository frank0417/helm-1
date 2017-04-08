/*
Copyright 2017 AppsCode Inc. All rights reserved.
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

package clientset

import (
	"fmt"

	schema "k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/apimachinery/registered"
	rest "k8s.io/kubernetes/pkg/client/restclient"
)

const (
	defaultAPIPath = "/apis"
)

type ExtensionInterface interface {
	RESTClient() rest.Interface
	ReleaseNamespacer
}

// ExtensionsClient is used to interact with experimental Kubernetes features.
// Features of Extensions group are not supported and may be changed or removed in
// incompatible ways at any time.
type ExtensionsClient struct {
	restClient rest.Interface
}

func (a *ExtensionsClient) Release(namespace string) ReleaseInterface {
	return newRelease(a, namespace)
}

// NewExtensions creates a new ExtensionsClient for the given config. This client
// provides access to experimental Kubernetes features.
// Features of Extensions group are not supported and may be changed or removed in
// incompatible ways at any time.
func NewExtensionsForConfig(c *rest.Config) (*ExtensionsClient, error) {
	config := *c
	if err := setExtensionsDefaults(&config); err != nil {
		return nil, err
	}
	client, err := rest.RESTClientFor(&config)
	if err != nil {
		return nil, err
	}
	return &ExtensionsClient{client}, nil
}

// NewExtensionsOrDie creates a new ExtensionsClient for the given config and
// panics if there is an error in the config.
// Features of Extensions group are not supported and may be changed or removed in
// incompatible ways at any time.
func NewExtensionsForConfigOrDie(c *rest.Config) *ExtensionsClient {
	client, err := NewExtensionsForConfig(c)
	if err != nil {
		panic(err)
	}
	return client
}

// New creates a new ExtensionsV1alpha1Client for the given RESTClient.
func NewNewExtensions(c rest.Interface) *ExtensionsClient {
	return &ExtensionsClient{c}
}

func setExtensionsDefaults(config *rest.Config) error {
	gv, err := schema.ParseGroupVersion("helm.sh/v1alpha1")
	if err != nil {
		return err
	}
	// if helm.sh/v1alpha1 is not enabled, return an error
	if !registered.IsEnabledVersion(gv) {
		return fmt.Errorf("helm.sh/v1alpha1 is not enabled")
	}
	config.APIPath = defaultAPIPath
	if config.UserAgent == "" {
		config.UserAgent = rest.DefaultKubernetesUserAgent()
	}

	if config.GroupVersion == nil || config.GroupVersion.Group != "helm.sh" {
		g, err := registered.Group("helm.sh")
		if err != nil {
			return err
		}
		copyGroupVersion := g.GroupVersion
		config.GroupVersion = &copyGroupVersion
	}

	config.NegotiatedSerializer = DirectCodecFactory{extendedCodec: ExtendedCodec}

	return nil
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *ExtensionsClient) RESTClient() rest.Interface {
	if c == nil {
		return nil
	}
	return c.restClient
}