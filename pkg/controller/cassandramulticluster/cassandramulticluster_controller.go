/*
Copyright 2018 The Multicluster-Controller Authors.

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

package cassandramulticluster

import (
	"context"
	"fmt"

	"admiralty.io/multicluster-controller/pkg/reconcile"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"

	"time"

	apicc "github.com/Orange-OpenSource/cassandra-k8s-operator/pkg/apis"
	ccv1 "github.com/Orange-OpenSource/cassandra-k8s-operator/pkg/apis/db/v1alpha1"
	apicmc "github.com/Orange-OpenSource/multicluster-cassandra-operator/pkg/apis"
	cmcv1 "github.com/Orange-OpenSource/multicluster-cassandra-operator/pkg/apis/multicluster/v1alpha1"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/imdario/mergo"

	"admiralty.io/multicluster-controller/pkg/cluster"
	"admiralty.io/multicluster-controller/pkg/controller"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
)

//Clusters defined each kubernetes we want to connect on it
type Clusters struct {
	Name    string
	Cluster *cluster.Cluster
}

//Clients is the k8s client to use to connect to each kubernetes
type Clients struct {
	name   string
	client client.Client
}

//reconciler is the base struc to be used for MultiCassKop
type reconciler struct {
	clients   []*Clients
	cmc       *cmcv1.CassandraMultiCluster
	namespace string
}

// NewController will create k8s clients for each k8s clusters,
// and watch for changes to CassandraMultiCluster and CassandraCluster CRD objects
func NewController(clusters []Clusters, namespace string) (*controller.Controller, error) {

	var clients []*Clients
	for i, value := range clusters {
		logrus.Infof("Create Client %d for Cluster %s", i+1, value.Name)
		client, err := value.Cluster.GetDelegatingClient()
		if err != nil {
			return nil, fmt.Errorf("getting delegating client %d for Cluster %s Cluster: %v", i, value.Name,
				err)
		}
		clients = append(clients, &Clients{value.Name, client})
		logrus.Infof("Add CRDs to Cluster %s Scheme", value.Name)
		if err := apicc.AddToScheme(value.Cluster.GetScheme()); err != nil {
			return nil, fmt.Errorf("adding APIs CassandraCluster to Cluster %s Cluster's scheme: %v", value.Name, err)
		}
		if err := apicmc.AddToScheme(value.Cluster.GetScheme()); err != nil {
			return nil, fmt.Errorf("adding APIs CassandraMultiCluster to Cluster %s Cluster's scheme: %v", value.Name,
				err)
		}
	}

	co := controller.New(&reconciler{clients: clients, namespace: namespace}, controller.Options{})

	for _, value := range clusters {
		logrus.Info("Configuring Watch for CassandraMultiCluster")
		if err := co.WatchResourceReconcileObject(value.Cluster, &cmcv1.CassandraMultiCluster{ObjectMeta: metav1.ObjectMeta{Namespace: namespace}},
			controller.WatchOptions{}); err != nil {
			return nil, fmt.Errorf("setting up CassandraMultiCluster watch in Cluster %s Cluster: %v", value.Name, err)
		}
	}
	return co, nil
}

func (r *reconciler) Reconcile(req reconcile.Request) (reconcile.Result, error) {
	//cc := &ccv1.CassandraCluster{}
	requeue30 := reconcile.Result{RequeueAfter: 30 * time.Second}
	requeue5 := reconcile.Result{RequeueAfter: 5 * time.Second}
	requeue := reconcile.Result{Requeue: true}
	forget := reconcile.Result{}

	if req.Namespace != r.namespace {
		logrus.Warningf("We don't watch the object in this namespace %s/%s", req.Name, req.Namespace)
		return forget, nil
	}

	logrus.Infof("Reconcile %v.", req)

	// Fetch the CassandraCluster instance
	// It is stored in the Cluster with index 0
	r.cmc = &cmcv1.CassandraMultiCluster{}
	cmc := r.cmc
	err := r.clients[0].client.Get(context.TODO(), req.NamespacedName, cmc)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			// ...TODO: multicluster garbage collector
			// Until then...
			// TODO: Need to manually garbage collector on Distant client.. This is safe enough ?? Warning!!!!
			r.deleteCassandraClusters(req.NamespacedName)

			return forget, nil
		}
		// Error reading the object - requeue the request.
		return requeue, err
	}

	var storedCC *ccv1.CassandraCluster
	for i, value := range r.clients {
		var cc *ccv1.CassandraCluster
		var found bool
		if found, cc = r.getCassandraClusterForContext(value.name); !found {
			logrus.Warningf("Cluster %s is not found in CassandraMultiCluster Specs", value.name)
			break

		}
		//Add defaults value for CassandraCluster if not set
		cc.CheckDefaults()

		cli := r.clients[i].client

		if storedCC, err = r.CreateOrUpdateCassandraCluster(cli, cc); err != nil {
			logrus.Info("error on CassandraCluster %s in Cluster ", cc.Name, value.name)
			return requeue5, err
		}

		if !r.ReadyCassandraCluster(storedCC) {
			logrus.Infof("CassandraCluster %s in Cluster %s not Ready, we wait. [phase=%s / action=%s / status=%s]",
				cc.Name, value.name, storedCC.Status.Phase, storedCC.Status.LastClusterAction,
				storedCC.Status.LastClusterActionStatus)
			return requeue30, err
		}
	}

	//TODO: Not sure if I can use forget or requeueXX here
	return requeue30, err
}

func (r *reconciler) namespacedName(name, namespace string) types.NamespacedName {
	return types.NamespacedName{
		Namespace: namespace,
		Name:      name,
	}
}

//getCassandraClusterForContext return the CassandraCluster object to create for the current context
//It merges the base definition, with the override part for the specified context in the CassandraMultiCluster CRD
func (r *reconciler) getCassandraClusterForContext(context string) (bool, *ccv1.CassandraCluster) {
	base := r.cmc.Spec.Base.DeepCopy()
	for cmcclName, override := range r.cmc.Spec.Override {
		if context == cmcclName {
			mergo.Merge(base, override, mergo.WithOverride)
			return true, base
		}
	}
	return false, nil
}

//Riskyyy
func (r *reconciler) deleteCassandraClusters(nsn types.NamespacedName) error {

	var err error
	for _, value := range r.clients {
		//var cc *ccv1.CassandraCluster
		//var found bool
		//cli := r.clients[i].client
		cli := value
		err = r.deleteCassandraCluster(nsn, cli)
		if err != nil {
			logrus.Errorf("Error deleting CassandraCluster err=%v", err)
		}
	}
	return err

}

func (r *reconciler) deleteCassandraCluster(nsn types.NamespacedName, cli *Clients) error {
	var cc *ccv1.CassandraCluster
	logrus.Infof("Delete %s in namespace %s of cluster %s", nsn.Name, nsn.Namespace, cli.name)
	if err := cli.client.Get(context.TODO(), nsn, cc); err != nil {
		if errors.IsNotFound(err) {
			// all good
			return nil
		}
		return err
	}
	if err := cli.client.Delete(context.TODO(), cc); err != nil {
		return err
	}
	return nil
}
