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
}

type Targets struct {
	// Schema
	// +required
	Schema Schema `json:"schema"`

	Defaults   Defaults    `json:"defaults"`
	Iterations []Iteration `json:"iterations,omitempty"`
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
	// Name
	// +required
	Name string `json:"name"`

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

	Failures []IterationFailure `json:"failures,omitempty"`

	DisabledIterations []DisabledIteration `json:"disabledIterations,omitempty"`
}

type IterationFailure struct {
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

// Konfiguration is the Schema for the konfigurations API.
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
