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

package v1

import (
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// WrityClusterSpec defines the desired state of WrityCluster
type WrityClusterSpec struct {
	Size             *int32            `json:"size,omitempty"`
	StorageSpec      *StorageSpec      `json:"storage,omitempty"`
	WritySpec        *WritySpec        `json:"writy,omitempty"`
	LoadBalancerSpec *LoadBalancerSpec `json:"loadbalancer,omitempty"`
}

type StorageSpec struct {
	ClaimName         string            `json:"claimName,omitempty"`
	Class             string            `json:"class,omitempty"`
	VolumeSizeRequest resource.Quantity `json:"volumeSizeRequest,omitempty"`
	VolumeSizeLimit   resource.Quantity `json:"volumeSizeLimit,omitempty"`
}

type WritySpec struct {
	Version  string `json:"version,omitempty"`
	Port     uint   `json:"port,omitempty"`
	LogLevel string `json:"logLevel,omitempty"`
}

type LoadBalancerSpec struct {
	Port     uint   `json:"port,omitempty"`
	LogLevel string `json:"logLevel,omitempty"`
}

// WrityClusterStatus defines the observed state of WrityCluster
type WrityClusterStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// WrityCluster is the Schema for the writyclusters API
type WrityCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   WrityClusterSpec   `json:"spec,omitempty"`
	Status WrityClusterStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// WrityClusterList contains a list of WrityCluster
type WrityClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []WrityCluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&WrityCluster{}, &WrityClusterList{})
}
