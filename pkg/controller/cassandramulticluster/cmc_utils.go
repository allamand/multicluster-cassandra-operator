package cassandramulticluster

import (
	"k8s.io/apimachinery/pkg/types"
	ccv1 "github.com/Orange-OpenSource/cassandra-k8s-operator/pkg/apis/db/v1alpha1"
)

func (r *reconciler) readyCassandraCluster(cc *ccv1.CassandraCluster) bool{
	return types.NamespacedName{
		Namespace: r.namespace,
		Name:      fmt.Sprintf("%s-%s", pod.Namespace, pod.Name),
	}
}
