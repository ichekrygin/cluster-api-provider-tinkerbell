/*
Copyright 2021 The Kubernetes Authors.

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

package v1beta1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	capierrors "sigs.k8s.io/cluster-api/errors"
)

const (
	// MachineFinalizer allows ReconcileTinkerbellMachine to clean up Tinkerbell resources before
	// removing it from the apiserver.
	MachineFinalizer = "tinkerbellmachine.infrastructure.cluster.x-k8s.io"
)

// TinkerbellMachineSpec defines the desired state of TinkerbellMachine.
type TinkerbellMachineSpec struct {
	// ImageLookupFormat is the URL naming format to use for machine images when
	// a machine does not specify. When set, this will be used for all cluster machines
	// unless a machine specifies a different ImageLookupFormat. Supports substitutions
	// for {{.BaseRegistry}}, {{.OSDistro}}, {{.OSVersion}} and {{.KubernetesVersion}} with
	// the basse URL, OS distribution, OS version, and kubernetes version, respectively.
	// BaseRegistry will be the value in ImageLookupBaseRegistry or ghcr.io/tinkerbell/cluster-api-provider-tinkerbell
	// (the default), OSDistro will be the value in ImageLookupOSDistro or ubuntu (the default),
	// OSVersion will be the value in ImageLookupOSVersion or default based on the OSDistro
	// (if known), and the kubernetes version as defined by the packages produced by
	// kubernetes/release: v1.13.0, v1.12.5-mybuild.1, or v1.17.3. For example, the default
	// image format of {{.BaseRegistry}}/{{.OSDistro}}-{{.OSVersion}}:{{.KubernetesVersion}}.gz will
	// attempt to pull the image from that location. See also: https://golang.org/pkg/text/template/
	// +optional
	ImageLookupFormat string `json:"imageLookupFormat,omitempty"`

	// ImageLookupBaseRegistry is the base Registry URL that is used for pulling images,
	// if not set, the default will be to use ghcr.io/tinkerbell/cluster-api-provider-tinkerbell.
	// +optional
	ImageLookupBaseRegistry string `json:"imageLookupBaseRegistry,omitempty"`

	// ImageLookupOSDistro is the name of the OS distro to use when fetching machine images,
	// if not set it will default to ubuntu.
	// +optional
	ImageLookupOSDistro string `json:"imageLookupOSDistro,omitempty"`

	// ImageLookupOSVersion is the version of the OS distribution to use when fetching machine
	// images. If not set it will default based on ImageLookupOSDistro.
	// +optional
	ImageLookupOSVersion string `json:"imageLookupOSVersion,omitempty"`

	// TemplateOverride overrides the default Tinkerbell template used by CAPT.
	// You can learn more about Tinkerbell templates here: https://docs.tinkerbell.org/templates/
	// +optional
	TemplateOverride string `json:"templateOverride,omitempty"`

	// Those fields are set programmatically, but they cannot be re-constructed from "state of the world", so
	// we put them in spec instead of status.
	HardwareName string `json:"hardwareName,omitempty"`
	ProviderID   string `json:"providerID,omitempty"`
}

// TinkerbellMachineStatus defines the observed state of TinkerbellMachine.
type TinkerbellMachineStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Ready is true when the provider resource is ready.
	// +optional
	Ready bool `json:"ready"`

	// Addresses contains the Tinkerbell device associated addresses.
	Addresses []corev1.NodeAddress `json:"addresses,omitempty"`

	// InstanceStatus is the status of the Tinkerbell device instance for this machine.
	// +optional
	InstanceStatus *TinkerbellResourceStatus `json:"instanceStatus,omitempty"`

	// Any transient errors that occur during the reconciliation of Machines
	// can be added as events to the Machine object and/or logged in the
	// controller's output.
	// +optional
	ErrorReason *capierrors.MachineStatusError `json:"errorReason,omitempty"`

	// ErrorMessage will be set in the event that there is a terminal problem
	// reconciling the Machine and will contain a more verbose string suitable
	// for logging and human consumption.
	//
	// This field should not be set for transitive errors that a controller
	// faces that are expected to be fixed automatically over
	// time (like service outages), but instead indicate that something is
	// fundamentally wrong with the Machine's spec or the configuration of
	// the controller, and that manual intervention is required. Examples
	// of terminal errors would be invalid combinations of settings in the
	// spec, values that are unsupported by the controller, or the
	// responsible controller itself being critically misconfigured.
	//
	// Any transient errors that occur during the reconciliation of Machines
	// can be added as events to the Machine object and/or logged in the
	// controller's output.
	// +optional
	ErrorMessage *string `json:"errorMessage,omitempty"`
}

// +kubebuilder:subresource:status
// +kubebuilder:object:root=true
// +kubebuilder:resource:path=tinkerbellmachines,scope=Namespaced,categories=cluster-api
// +kubebuilder:storageversion
// +kubebuilder:printcolumn:name="Cluster",type="string",JSONPath=".metadata.labels.cluster\\.x-k8s\\.io/cluster-name",description="Cluster to which this TinkerbellMachine belongs"
// +kubebuilder:printcolumn:name="State",type="string",JSONPath=".status.instanceState",description="Tinkerbell instance state"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.ready",description="Machine ready status"
// +kubebuilder:printcolumn:name="InstanceID",type="string",JSONPath=".spec.providerID",description="Tinkerbell instance ID"
// +kubebuilder:printcolumn:name="Machine",type="string",JSONPath=".metadata.ownerReferences[?(@.kind==\"Machine\")].name",description="Machine object which owns with this TinkerbellMachine"

// TinkerbellMachine is the Schema for the tinkerbellmachines API.
type TinkerbellMachine struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TinkerbellMachineSpec   `json:"spec,omitempty"`
	Status TinkerbellMachineStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// TinkerbellMachineList contains a list of TinkerbellMachine.
type TinkerbellMachineList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TinkerbellMachine `json:"items"`
}

//nolint:gochecknoinits
func init() {
	SchemeBuilder.Register(&TinkerbellMachine{}, &TinkerbellMachineList{})
}
