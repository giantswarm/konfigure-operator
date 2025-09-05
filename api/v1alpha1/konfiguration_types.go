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

// KonfigurationSpec defines the desired state of the Konfiguration.
type KonfigurationSpec struct {
	// Define what konfigurations to render.
	// +required
	Targets Targets `json:"targets"`

	// Defines where and how to store the rendered konfigurations.
	// +required
	Destination Destination `json:"destination"`

	// Defines how to reconcile the Konfiguration.
	Reconciliation Reconciliation `json:"reconciliation,omitempty"`

	// Defines where to find the source of the konfiguration that needs to be rendered.
	Sources Sources `json:"sources"`
}

// Targets define information on what konfiguration to render and how to render them.
type Targets struct {
	// Defines where to locate the KonfigurationSchema.
	// +required
	Schema Schema `json:"schema"`

	// Define default inputs to be used for every single iteration.
	// Individual iterations may override default inputs.
	Defaults Defaults `json:"defaults"`

	// Defines what konfigurations to render. A single reconciliation loop iterates over each entry
	// and renders the konfiguration, wraps them to Kubernetes manifests and enforces the state of those in the cluster.
	Iterations map[string]Iteration `json:"iterations,omitempty"`
}

// Schema defines information on what konfiguration schema to use to render the target konfigurations.
type Schema struct {
	// Defines where to locate the KonfigurationSchema as a Kubernetes manifest.
	Reference SchemaReference `json:"reference"`
}

// SchemaReference defines information on how to locate a KonfigurationSchema as a Kubernetes manifest.
type SchemaReference struct {
	// Defines the .metadata.name of the KonfigurationSchema Kubernetes manifest to fetch.
	Name string `json:"name"`

	// Defines the .metadata.namespace of the KonfigurationSchema Kubernetes manifest to fetch.
	Namespace string `json:"namespace"`
}

// Defaults define information shared across iteration to render konfigurations.
type Defaults struct {
	// Defines default variable inputs to be used for every single iteration.
	Variables []NameValuePair `json:"variables,omitempty"`
}

// Iteration defines information needed to a single konfiguration to render.
type Iteration struct {
	// Defines variable inputs specific for the given iteration.
	// These variables are merged on top of the default variables, and thus may choose to override default ones.
	Variables []NameValuePair `json:"variables,omitempty"`
}

// NameValuePair is a simple structure for defining input fields by name and value.
type NameValuePair struct {
	// Name of the input.
	// +required
	Name string `json:"name"`

	// Value of the input.
	// +required
	Value string `json:"value"`
}

// Destination defines where and how to store the rendered konfigurations.
type Destination struct {
	// Defines the namespace where the rendered Kubernetes manifests will be applied.
	// +required
	Namespace string `json:"namespace"`

	// Defines rules on how to name the rendered Kubernetes manifests.
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

// NamingOptions defines rules on how to name the rendered Kubernetes manifests.
// The core of the name is always the iteration name, the map key under .spec.targets.iterations.
type NamingOptions struct {
	// Prefix is prepended at the beginning of the iteration name.
	// +kubebuilder:validation:Type=string
	// +kubebuilder:validation:Pattern="^[a-z0-9]([-a-z0-9]{0,62}[a-z0-9])?$"
	Prefix string `json:"prefix,omitempty"`

	// Suffix is appended to the end of the iteration name.
	// +kubebuilder:validation:Type=string
	// +kubebuilder:validation:Pattern="^[a-z0-9]([-a-z0-9]{0,62}[a-z0-9])?$"
	Suffix string `json:"suffix,omitempty"`

	// UseSeparator indicates whether to separate the iteration name
	// from the prefix and/or suffix with a single `-` character.
	// +kubebuilder:default:=true
	// +optional
	UseSeparator bool `json:"useSeparator,omitempty"`
}

// Reconciliation defines how to reconcile the Konfiguration.
type Reconciliation struct {
	// The interval at which to reconcile the Konfiguration.
	// +kubebuilder:validation:Type=string
	// +kubebuilder:validation:Pattern="^([0-9]+(\\.[0-9]+)?(ms|s|m|h))+$"
	// +required
	Interval metav1.Duration `json:"interval"`

	// The interval at which to retry a previously failed reconciliation.
	// +kubebuilder:validation:Type=string
	// +kubebuilder:validation:Pattern="^([0-9]+(\\.[0-9]+)?(ms|s|m|h))+$"
	// +optional
	RetryInterval *metav1.Duration `json:"retryInterval,omitempty"`

	// This flag tells the controller to suspend rendering the Konfiguration.
	// It does not apply to already started executions. Default is false.
	// +kubebuilder:default:=false
	// +optional
	Suspend bool `json:"suspend,omitempty"`
}

// Sources define where to find the source of the konfiguration that needs to be rendered.
type Sources struct {
	// Defines to locate the source of the konfiguration structure as a Flux source.
	Flux FluxSource `json:"flux,omitempty"`
}

// FluxSource defines supported Flux sources as Konfiguration sources.
type FluxSource struct {
	// Defines the source as a Flux GitRepository manifest.
	// +required
	GitRepository FluxSourceGitRepository `json:"gitRepository"`
}

// FluxSourceGitRepository defines information to reference a Flux GitRepository as the Konfiguration source.
type FluxSourceGitRepository struct {
	// Name of the referenced Flux GitRepository resource.
	// +required
	Name string `json:"name"`

	// Namespace of the referenced Flux GitRepository resource.
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

	// The last time the Konfiguration attempted reconciliation.
	// +optional
	LastReconciledAt string `json:"lastReconciledAt,omitempty"`

	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// The list of failed iterations during the last full reconciliation.
	Failed []FailedIteration `json:"failed,omitempty"`

	// The list of rendered manifests that were not applied during the last full reconciliation,
	// because their reconciliation is disabled via the `configuration.giantswarm.io/reconcile: disabled` label.
	Disabled []DisabledIteration `json:"disabled,omitempty"`
}

// FailedIteration defines information of a single failed iteration.
type FailedIteration struct {
	// The name of the iteration, that is the map key under .spec.targets.iterations.
	// +kubebuilder:validation:Type=string
	// +required
	Name string `json:"appName"`

	// Informational message on the cause of the failure.
	// +kubebuilder:validation:Type=string
	// +required
	Message string `json:"message"`
}

// DisabledIteration defines information on managed Kubernetes manifests whose state is not currently enforced
// because they are disabled for reconciliation.
type DisabledIteration struct {
	// The name of the iteration, that is the map key under .spec.targets.iterations.
	// +kubebuilder:validation:Type=string
	// +required
	Name string `json:"appName"`

	// The kind of the iteration from the schema, generally ConfigMap or Secret.
	// +kubebuilder:validation:Type=string
	// +required
	Kind string `json:"kind"`

	// Reference to the target manifest.
	// +required
	Target DisabledIterationTarget `json:"target"`
}

// DisabledIterationTarget defines reference of a single reconciliation-disabled managed Kubernetes resource.
type DisabledIterationTarget struct {
	// Name of the resource.
	// +required
	Name string `json:"name"`

	// Namespace of the resource.
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
