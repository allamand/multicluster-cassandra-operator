package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apicc "github.com/Orange-OpenSource/cassandra-k8s-operator/pkg/apis/db/v1alpha1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// CassandraMultiClusterSpec defines the desired state of CassandraMultiCluster
// +k8s:openapi-gen=true
type CassandraMultiClusterSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book.kubebuilder.io/beyond_basics/generating_crd.html
	CassandraCluster []apicc.CassandraCluster `json:"cassandraCluster,omitempty"`
}

// CassandraMultiClusterStatus defines the observed state of CassandraMultiCluster
// +k8s:openapi-gen=true
type CassandraMultiClusterStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book.kubebuilder.io/beyond_basics/generating_crd.html
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CassandraMultiCluster is the Schema for the cassandramulticlusters API
// +k8s:openapi-gen=true
type CassandraMultiCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CassandraMultiClusterSpec   `json:"spec,omitempty"`
	Status CassandraMultiClusterStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CassandraMultiClusterList contains a list of CassandraMultiCluster
type CassandraMultiClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CassandraMultiCluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&CassandraMultiCluster{}, &CassandraMultiClusterList{})
}
