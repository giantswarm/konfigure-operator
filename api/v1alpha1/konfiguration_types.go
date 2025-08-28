/*
Copyright 2025.

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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

const (
	KonfigureOperatorFinalizer = "finalizers.giantswarm.io/konfigure-operator"
)

// KonfigurationSpec defines the desired state of Konfiguration.
type KonfigurationSpec struct {
	// Targets
	// +required
	Targets Targets `json:"targets"`

	// Destination
	// +required
	Destination Destination `json:"destination"`

	// Reconciliation
	Reconciliation Reconciliation `json:"reconciliation,omitempty"`

	// Sources
	Sources Sources `json:"sources"`
}

type Targets struct {
	// Schema
	// +required
	Schema Schema `json:"schema"`

	Defaults   Defaults             `json:"defaults"`
	Iterations map[string]Iteration `json:"iterations,omitempty"`
}

type Schema struct {
	// Reference
	Reference SchemaReference `json:"reference"`
}

type SchemaReference struct {
	// Name
	Name string `json:"name"`

	// Namespace
	Namespace string `json:"namespace"`
}

type Defaults struct {
	Variables []NameValuePair `json:"variables,omitempty"`
}

type Iteration struct {
	Variables []NameValuePair `json:"variables,omitempty"`
}

type NameValuePair struct {
	// Name
	// +required
	Name string `json:"name"`

	// Value
	// +required
	Value string `json:"value"`
}

type Destination struct {
	// +required
	Namespace string `json:"namespace"`
	// +required
	Naming NamingOptions `json:"naming"`
}

func (n *NamingOptions) Render(core string) string {
	name := core

	separator := ""
	if n.UseSeparator {
		separator = "-"
	}

	if n.Prefix != "" {
		name = n.Prefix + separator + name
	}

	if n.Suffix != "" {
		name = name + separator + n.Suffix
	}

	return name
}

type NamingOptions struct {
	// +kubebuilder:validation:Type=string
	// +kubebuilder:validation:Pattern="^[a-z0-9]([-a-z0-9]{0,62}[a-z0-9])?$"
	Prefix string `json:"prefix,omitempty"`
	// +kubebuilder:validation:Type=string
	// +kubebuilder:validation:Pattern="^[a-z0-9]([-a-z0-9]{0,62}[a-z0-9])?$"
	Suffix string `json:"suffix,omitempty"`
	// +kubebuilder:default:=true
	// +optional
	UseSeparator bool `json:"useSeparator,omitempty"`
}

type Reconciliation struct {
	// +kubebuilder:validation:Type=string
	// +kubebuilder:validation:Pattern="^([0-9]+(\\.[0-9]+)?(ms|s|m|h))+$"
	// +required
	Interval metav1.Duration `json:"interval"`

	// +kubebuilder:validation:Type=string
	// +kubebuilder:validation:Pattern="^([0-9]+(\\.[0-9]+)?(ms|s|m|h))+$"
	// +optional
	RetryInterval *metav1.Duration `json:"retryInterval,omitempty"`

	// +kubebuilder:default:=false
	// +optional
	Suspend bool `json:"suspend,omitempty"`
}

type Sources struct {
	Flux FluxSource `json:"flux,omitempty"`
}

type FluxSource struct {
	// +required
	GitRepository FluxSourceGitRepository `json:"gitRepository"`
}

type FluxSourceGitRepository struct {
	// +required
	Name string `json:"name"`
	// +required
	Namespace string `json:"namespace"`
}

// KonfigurationStatus defines the observed state of Konfiguration.
type KonfigurationStatus struct {
	// ObservedGeneration is the last observed generation.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// The last successfully applied revision.
	// Equals the Revision of the applied artifact from the referenced source.
	// +optional
	LastAppliedRevision string `json:"lastAppliedRevision,omitempty"`

	// The last revision that was attempted for reconciliation.
	// Equals the Revision of the last attempted artifact from the referenced source.
	// +optional
	LastAttemptedRevision string `json:"lastAttemptedRevision,omitempty"`

	// +optional
	LastReconciledAt string `json:"lastReconciledAt,omitempty"`

	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	Failed []FailedIteration `json:"failed,omitempty"`

	Disabled []DisabledIteration `json:"disabled,omitempty"`
}

type FailedIteration struct {
	// +kubebuilder:validation:Type=string
	// +required
	Name string `json:"appName"`

	// +kubebuilder:validation:Type=string
	// +required
	Message string `json:"message"`
}

type DisabledIteration struct {
	// +kubebuilder:validation:Type=string
	// +required
	Name string `json:"appName"`

	// +kubebuilder:validation:Type=string
	// +required
	Kind string `json:"kind"`

	// +required
	Target DisabledIterationTarget `json:"target"`
}

type DisabledIterationTarget struct {
	// +required
	Name string `json:"name"`

	// +required
	Namespace string `json:"namespace"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=kfg

// Konfiguration is the Schema for the konfigurations API.
// +kubebuilder:printcolumn:name="Destination",type="string",JSONPath=".spec.destination.namespace",description=""
// +kubebuilder:printcolumn:name="Prefix",type="string",JSONPath=".spec.destination.naming.prefix",description=""
// +kubebuilder:printcolumn:name="Suffix",type="string",JSONPath=".spec.destination.naming.suffix",description=""
// +kubebuilder:printcolumn:name="UseSeparator",type="boolean",JSONPath=".spec.destination.naming.useSeparator",description=""
// +kubebuilder:printcolumn:name="SchemaName",type="string",JSONPath=".spec.targets.schema.reference.name",description=""
// +kubebuilder:printcolumn:name="SchemaNamespace",type="string",JSONPath=".spec.targets.schema.reference.namespace",description=""
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description=""
// +kubebuilder:printcolumn:name="Suspended",type="boolean",JSONPath=".spec.reconciliation.suspend",description=""
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].status",description=""
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].message",description=""
type Konfiguration struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KonfigurationSpec   `json:"spec,omitempty"`
	Status KonfigurationStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// KonfigurationList contains a list of Konfiguration.
type KonfigurationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Konfiguration `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Konfiguration{}, &KonfigurationList{})
}
