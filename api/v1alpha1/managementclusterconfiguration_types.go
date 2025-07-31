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

const (
	KonfigureOperatorFinalizer = "finalizers.giantswarm.io/konfigure-operator"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ManagementClusterConfigurationSpec defines the desired state of ManagementClusterConfiguration.
type ManagementClusterConfigurationSpec struct {
	// Sources
	Sources Sources `json:"sources"`

	// Destination
	// +required
	Destination Destination `json:"destination"`

	// Configuration
	Configuration Configuration `json:"configuration,omitempty"`

	// Reconciliation
	Reconciliation Reconciliation `json:"reconciliation,omitempty"`
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

type Configuration struct {
	// +required
	Cluster      ClusterConfiguration      `json:"cluster"`
	Applications ApplicationsConfiguration `json:"applications,omitempty"`
}

type ClusterConfiguration struct {
	// +required
	Name string `json:"name"`
}

type ApplicationsConfiguration struct {
	Includes ApplicationMatchers `json:"includes,omitempty"`
	Excludes ApplicationMatchers `json:"excludes,omitempty"`
}

type ApplicationMatchers struct {
	ExactMatchers []string `json:"exactMatchers,omitempty"`
	RegexMatchers []string `json:"regexMatchers,omitempty"`
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

// ManagementClusterConfigurationStatus defines the observed state of ManagementClusterConfiguration.
type ManagementClusterConfigurationStatus struct {
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
	Failures []FailureStatus `json:"failures,omitempty"`

	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// +optional
	Misses []string `json:"misses,omitempty"`

	// +optional
	DisabledReconciles []DisabledReconcile `json:"disabledReconciles,omitempty"`
}

type FailureStatus struct {
	// +kubebuilder:validation:Type=string
	// +required
	AppName string `json:"appName"`

	// +kubebuilder:validation:Type=string
	// +required
	Message string `json:"message"`
}

type DisabledReconcile struct {
	// +kubebuilder:validation:Type=string
	// +required
	AppName string `json:"appName"`

	// +kubebuilder:validation:Type=string
	// +required
	Kind string `json:"kind"`

	// +required
	Target DisabledReconcileTarget `json:"target"`
}

type DisabledReconcileTarget struct {
	// +required
	Name string `json:"name"`
	// +required
	Namespace string `json:"namespace"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=mcc

// ManagementClusterConfiguration is the Schema for the managementclusterconfigurations API.
// +kubebuilder:printcolumn:name="Cluster",type="string",JSONPath=".spec.configuration.cluster.name",description=""
// +kubebuilder:printcolumn:name="Destination",type="string",JSONPath=".spec.destination.namespace",description=""
// +kubebuilder:printcolumn:name="Prefix",type="string",JSONPath=".spec.destination.naming.prefix",description=""
// +kubebuilder:printcolumn:name="Suffix",type="string",JSONPath=".spec.destination.naming.suffix",description=""
// +kubebuilder:printcolumn:name="UseSeparator",type="boolean",JSONPath=".spec.destination.naming.useSeparator",description=""
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description=""
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].status",description=""
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].message",description=""
type ManagementClusterConfiguration struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ManagementClusterConfigurationSpec   `json:"spec,omitempty"`
	Status ManagementClusterConfigurationStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ManagementClusterConfigurationList contains a list of ManagementClusterConfiguration.
type ManagementClusterConfigurationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ManagementClusterConfiguration `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ManagementClusterConfiguration{}, &ManagementClusterConfigurationList{})
}
