package cassandramulticluster

import (
	"context"
	ccv1 "github.com/Orange-OpenSource/cassandra-k8s-operator/pkg/apis/db/v1alpha1"
	"github.com/kylelemons/godebug/pretty"
	"github.com/sirupsen/logrus"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *reconciler) ReadyCassandraCluster(cc *ccv1.CassandraCluster) bool{
	if cc.Status.Phase != ccv1.ClusterPhaseRunning || cc.Status.LastClusterActionStatus != ccv1.StatusDone {
		return false
	}
	return true
}

func (r *reconciler) CreateOrUpdateCassandraCluster(client client.Client, cc *ccv1.CassandraCluster) (*ccv1.CassandraCluster, error) {
	storedCC := &ccv1.CassandraCluster{}
	if err := client.Get(context.TODO(), r.namespacedName(cc.Name, cc.Namespace), storedCC); err != nil {
		if errors.IsNotFound(err) {
			return r.CreateCassandraCluster(client, cc)
		}
		return storedCC, err
	}
	if !apiequality.Semantic.DeepEqual(storedCC.Spec, cc.Spec){
	//if !reflect.DeepEqual(storedCC.Spec, cc.Spec){
	logrus.Infof("Template is different: " + pretty.Compare(storedCC.Spec, cc.Spec))
	storedCC.Spec = cc.Spec
		return r.UpdateCassandraCluster(client, storedCC)
	}
	return storedCC, nil
}

func (r *reconciler) CreateCassandraCluster(client client.Client, cc *ccv1.CassandraCluster) (*ccv1.CassandraCluster, error) {
	var err error
	if err = client.Create(context.TODO(), cc); err != nil{
		if errors.IsAlreadyExists(err) {
			return cc, nil
		}
	}
	return cc, err
}

func (r *reconciler) UpdateCassandraCluster(client client.Client, cc *ccv1.CassandraCluster) (*ccv1.CassandraCluster, error) {
	var err error
	if err = client.Update(context.TODO(), cc); err != nil{
		if errors.IsAlreadyExists(err) {
			return cc, nil
		}
	}
	return cc, err
}