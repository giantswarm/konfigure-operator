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

// ManagementClusterConfigurationSpec defines the desired state of ManagementClusterConfiguration.
type ManagementClusterConfigurationSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Sources
	Sources Sources `json:"sources"`

	// Encryption
	Encryption Encryption `json:"encryption,omitempty"`

	// Destination
	Destination Destination `json:"destination,omitempty"`

	// Configuration
	Configuration Configuration `json:"configuration,omitempty"`

	// Reconciliation
	Reconciliation Reconciliation `json:"reconciliation,omitempty"`
}

type Sources struct {
	Flux FluxSource `json:"flux,omitempty"`
}

type FluxSource struct {
	Service       FluxSourceService       `json:"service"`
	GitRepository FluxSourceGitRepository `json:"gitRepository"`
}

type FluxSourceService struct {
	Url string `json:"url"`
}

type FluxSourceGitRepository struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

type Encryption struct {
	Sops SopsEncryption `json:"sops,omitempty"`
}

type SopsEncryption struct {
	KeysDirectory string `json:"keysDirectory,omitempty"`
}

type Destination struct {
	Namespace string `json:"namespace"`
}

type Configuration struct {
	Cluster      ClusterConfiguration      `json:"cluster,omitempty"`
	Applications ApplicationsConfiguration `json:"applications,omitempty"`
}

type ClusterConfiguration struct {
	Name string `json:"name"`
}

type ApplicationsConfiguration struct {
	RegexMatchers []string `json:"regexMatchers,omitempty"`
	ExactMatchers []string `json:"exactMatchers,omitempty"`
}

type Reconciliation struct {
	Interval metav1.Duration `json:"interval"`
}

// ManagementClusterConfigurationStatus defines the observed state of ManagementClusterConfiguration.
type ManagementClusterConfigurationStatus struct {
	// ObservedGeneration is the last observed generation.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// +optional
	LastAppliedRevision string `json:"lastAppliedRevision,omitempty"`

	// +optional
	LastAttemptedRevision string `json:"lastAttemptedRevision,omitempty"`

	// +optional
	LastHandledReconcileAt string `json:"lastHandledReconcileAt,omitempty"`

	// +optional
	Failures []FailureStatus `json:"failures,omitempty"`

	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

type FailureStatus struct {
	ApiVersion string `json:"apiVersion,omitempty"`
	Kind       string `json:"kind,omitempty"`
	Name       string `json:"name,omitempty"`
	Namespace  string `json:"namespace,omitempty"`
	Message    string `json:"message,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=mcc

// ManagementClusterConfiguration is the Schema for the managementclusterconfigurations API.
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
