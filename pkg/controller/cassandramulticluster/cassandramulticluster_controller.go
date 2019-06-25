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
	"admiralty.io/multicluster-controller/pkg/reconcile"
	"context"
	"fmt"
	apicc "github.com/Orange-OpenSource/cassandra-k8s-operator/pkg/apis"
	ccv1 "github.com/Orange-OpenSource/cassandra-k8s-operator/pkg/apis/db/v1alpha1"
	apicmc "github.com/Orange-OpenSource/multicluster-cassandra-operator/pkg/apis"
	cmcv1 "github.com/Orange-OpenSource/multicluster-cassandra-operator/pkg/apis/multicluster/v1alpha1"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"

	"admiralty.io/multicluster-controller/pkg/cluster"
	"admiralty.io/multicluster-controller/pkg/controller"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
)

type ClustersConf struct {
	index int
	cluster *cluster.Cluster
	//clients *client.Client
}

type reconciler struct {
	clients   map[string]client.Client
	cmc       *cmcv1.CassandraMultiCluster
	namespace string
}

func NewController(clusters map[string]ClustersConf, namespace string) (*controller.Controller, error) {
	var clients = map[string]client.Client
	for key, value := range clusters {
		logrus.Info("Create Client %d for cluster %s", value.index, key)
		client, err := value.cluster.GetDelegatingClient()
		if err != nil {
			return nil, fmt.Errorf("getting delegating client %d for cluster %s cluster: %v", value.index, key, err)
		}

		clients[key] = client

		logrus.Infof("Add CRDs to cluster %s Scheme", key)
		if err := apicc.AddToScheme(value.cluster.GetScheme()); err != nil {
			return nil, fmt.Errorf("adding APIs CassandraCluster to cluster %s cluster's scheme: %v", key, err)
		}
		if err := apicmc.AddToScheme(value.cluster.GetScheme()); err != nil {
			return nil, fmt.Errorf("adding APIs CassandraMultiCluster to cluster %s cluster's scheme: %v", key, err)
		}

	}

	co := controller.New(&reconciler{clients: clients, namespace: namespace}, controller.Options{})

	for key, value := range clusters {
		//Demande au controlleur de faire un Watch des ressources de type Pod
		logrus.Info("Configuring Watch for CassandraMultiCluster")
		if err := co.WatchResourceReconcileObject(value.cluster, &cmcv1.CassandraMultiCluster{ObjectMeta: metav1.ObjectMeta{Namespace: namespace}},
			controller.WatchOptions{}); err != nil {
			return nil, fmt.Errorf("setting up CassandraMultiCluster watch in cluster %s cluster: %v", key, err)
		}

		// Note: At the moment, all clients share the same scheme under the hood
		// (k8s.io/client-go/kubernetes/scheme.Scheme), yet multicluster-controller gives each cluster a scheme pointer.
		// Therefore, if we needed a custom resource in multiple clients, we would redundantly
		// add it to each cluster's scheme, which points to the same underlying scheme.

		//SEB: TODO - pas sur de comprendre a quoi sert celui la ??
		if err := co.WatchResourceReconcileController(value.cluster, &cmcv1.CassandraMultiCluster{},
			controller.WatchOptions{}); err != nil {
			return nil, fmt.Errorf("setting up CassandraMultiCluster watch in cluster %s cluster: %v", key, err)
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

	if req.Namespace != r.namespace{
		return reconcile.Result{}, nil
	}


	logrus.Infof("Reconcile %v.", req)

	// Fetch the CassandraCluster instance
	r.cmc = &cmcv1.CassandraMultiCluster{}
	cmc := r.cmc
	err := r.cluster1.Get(context.TODO(), req.NamespacedName, cmc)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			// ...TODO: multicluster garbage collector
			// Until then...
			// TODO: Need to manually garbage collector on Distant clients.. This is safe enough ?? Warning!!!!
			return forget, nil
		}
		// Error reading the object - requeue the request.
		return forget, err
	}



	cc1 := &ccv1.CassandraCluster{}
	if err := r.cluster1.Get(context.TODO(), r.namespacedName(cmc.Spec.CassandraCluster[0].Name, cmc.Spec.CassandraCluster[0].Namespace), cc1); err != nil {
		if errors.IsNotFound(err) {
			err := r.cluster1.Create(context.TODO(), &cmc.Spec.CassandraCluster[0])
			return requeue5, err
		}
	}

	if cc1.Status.Phase != ccv1.ClusterPhaseRunning || cc1.Status.LastClusterActionStatus != ccv1.StatusDone{
		logrus.Infof("Cluster 1 not Ready, we wait. [phase=%s / action=%s / status=%s]", cc1.Status.Phase, cc1.Status.LastClusterAction, cc1.Status.LastClusterActionStatus)
		return requeue30, err
	}

	cc2 := &ccv1.CassandraCluster{}
	if err := r.cluster2.Get(context.TODO(), r.namespacedName(cmc.Spec.CassandraCluster[1].Name,
		cmc.Spec.CassandraCluster[1].Namespace), cc2); err != nil {
		if errors.IsNotFound(err) {
			err := r.cluster2.Create(context.TODO(), &cmc.Spec.CassandraCluster[1])
			return requeue5, err
		}
	}

	/*
	if reflect.DeepEqual(dg.Spec, og.Spec) {
		return reconcile.Result{}, nil
	}

	og.Spec = dg.Spec
	err := r.cluster2.Update(context.TODO(), og)
	return reconcile.Result{}, err
	*/
	return requeue, err
}

func (r *reconciler) cassandraMultiClusterNamespacedName(reqNS types.NamespacedName) types.NamespacedName {
	return types.NamespacedName{
		Namespace: r.namespace,
		Name:      reqNS.Name,
	}
}

func (r *reconciler) namespacedName(name, namespace string) types.NamespacedName {
	return types.NamespacedName{
		Namespace: namespace,
		Name:      name,
	}
}


func (r *reconciler) ghostNamespacedName(pod types.NamespacedName) types.NamespacedName {
	return types.NamespacedName{
		Namespace: r.namespace,
		Name:      fmt.Sprintf("%s-%s", pod.Namespace, pod.Name),
	}
}

func (r *reconciler) deleteCassandraCluster(nsn types.NamespacedName) error {
	cc := &ccv1.CassandraCluster{}
	if err := r.cluster2.Get(context.TODO(), nsn, cc); err != nil {
		if errors.IsNotFound(err) {
			// all good
			return nil
		}
		return err
	}
	if err := r.cluster2.Delete(context.TODO(), cc); err != nil {
		return err
	}
	return nil
}

/*
func (r *reconciler) makeCassandraCluster(cc *v1.Pod) *cmcv1.CassandraMultiCluster {
	return &cmcv1.CassandraMultiCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: r.namespace,
			Name:      fmt.Sprintf("%s-%s", pod.Namespace, pod.Name),
		},
	}
}
*/