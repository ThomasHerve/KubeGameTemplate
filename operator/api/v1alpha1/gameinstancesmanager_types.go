/*
Copyright 2026.

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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// GameInstancesManagerSpec defines the desired state of GameInstancesManager.
type GameInstancesManagerSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Foo is an example field of GameInstancesManager. Edit gameinstancesmanager_types.go to remove/update
	Frontend FrontendSpec `json:"frontend"`

	Hostname string `json:"hostname"`

	// Configuration of game instances.
	Instance InstanceSpec `json:"instance"`

	// Nouvelle section: Configuration des routes
    Routes *RoutesConfig `json:"routes,omitempty"`

	// Maximum number of concurrent game instances. If not set, there is no limit.
	// +optional
	// +kubebuilder:validation:Minimum=1
	MaxInstances *int32 `json:"maxInstances,omitempty"`
}

type InstanceSpec struct {

	// Container image repository.
	// +kubebuilder:validation:MinLength=1
	Repository string `json:"repository"`

	// Container image tag.
	// +optional
	// +kubebuilder:default="latest"
	Tag string `json:"tag,omitempty"`

	// Image pull policy.
	// +optional
	// +kubebuilder:default="Always"
	PullPolicy corev1.PullPolicy `json:"pullPolicy,omitempty"`

	// Port exposed by the container.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	InternalPort int32 `json:"internalPort"`
}

type RoutesConfig struct {
    // Route pour créer des instances
    // +optional
    // +kubebuilder:default=true
    CreateEnabled bool `json:"createEnabled,omitempty"`
    
    // Route pour supprimer des instances
    // +optional
    // +kubebuilder:default=false
    DeleteEnabled bool `json:"deleteEnabled,omitempty"`
}

type FrontendSpec struct {
	// +optional
	// +kubebuilder:default=true
	Enabled bool `json:"enabled,omitempty"`

	// Image repository, e.g. ghcr.io/org/frontend
	// +optional
	Repository string `json:"repository"`
 
	// Image tag. Defaults to "latest" if empty.
	// +optional
	Tag string `json:"tag,omitempty"`
 
	// +optional
	// +kubebuilder:default=IfNotPresent
	PullPolicy corev1.PullPolicy `json:"pullPolicy,omitempty"`
 
	// Number of replicas. Left nil-friendly for future autoscaling support
	// (equivalent to the Helm `{{- if not autoscaling.enabled }}` guard).
	// +optional
	// +kubebuilder:default=1
	Replicas *int32 `json:"replicas,omitempty"`
 
	// Container port exposed by the frontend
	// +optional
	// +kubebuilder:default=8080
	Port int32 `json:"port,omitempty"`
	
	// +optional
	// +kubebuilder:default=80
	ExternalPort int32 `json:"externalport,omitempty"`	
 
	// Backend protocol, defaults to https
	// +optional
	// +kubebuilder:default=https
	BackendProtocol string `json:"backendProtocol,omitempty"`
 
	// Whether HTTPRoute is enabled (exposed as HTTP_ROUTE_ENABLED env var)
	// +optional
	HTTPRouteEnabled bool `json:"httpRouteEnabled,omitempty"`
 
	// Optional: override the ServiceAccount name. If empty, the operator
	// derives it from the GameInstancesManager name (<name>-sa).
	// +optional
	ServiceAccountName string `json:"serviceAccountName,omitempty"`
 
	// Pod-level security context. Left free-form so callers can satisfy
	// either OpenShift's restricted SCC or a vanilla Kubernetes policy.
	// +optional
	PodSecurityContext *corev1.PodSecurityContext `json:"podSecurityContext,omitempty"`
 
	// +optional
	SecurityContext *corev1.SecurityContext `json:"securityContext,omitempty"`
 
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`
}

// GameInstancesManagerStatus defines the observed state of GameInstancesManager.
type GameInstancesManagerStatus struct {
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
 
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// Track the last applied routes configuration to detect changes
	// +optional
	LastAppliedRoutesState *RoutesConfig `json:"lastAppliedRoutesState,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`

// GameInstancesManager is the Schema for the gameinstancesmanagers API.
type GameInstancesManager struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GameInstancesManagerSpec   `json:"spec,omitempty"`
	Status GameInstancesManagerStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// GameInstancesManagerList contains a list of GameInstancesManager.
type GameInstancesManagerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GameInstancesManager `json:"items"`
}

func init() {
	SchemeBuilder.Register(&GameInstancesManager{}, &GameInstancesManagerList{})
}
