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

package main

import (
	"admiralty.io/multicluster-controller/pkg/cluster"
	"admiralty.io/multicluster-controller/pkg/manager"
	"admiralty.io/multicluster-service-account/pkg/config"
	"flag"
	"github.com/Orange-OpenSource/multicluster-cassandra-operator/pkg/controller/cassandramulticluster"
	"github.com/operator-framework/operator-sdk/pkg/k8sutil"
	"github.com/sirupsen/logrus"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/sample-controller/pkg/signals"
	"log"
)



func main() {
	flag.Parse()
	if flag.NArg() < 1 {
		log.Fatalf("Usage: CassandraMultiCluster cluster-1 cluster-2 .. cluster-n")
	}

	var clusters []cassandramulticluster.Clusters

	for i:=0 ; i < flag.NArg(); i++{
		clusterName := flag.Arg(i)
		logrus.Infof("Configuring Client %d for cluster %s.", i+1, clusterName)
		cfg, _, err := config.NamedConfigAndNamespace(clusterName)
		if err != nil {
			log.Fatal(err)
		}
		clusters = append(clusters,
			cassandramulticluster.Clusters{clusterName,cluster.New(clusterName, cfg, cluster.Options{})})

	}

	namespace, err := k8sutil.GetWatchNamespace()
	if err != nil {
		logrus.Fatalf("failed to get watch namespace: %v", err)
	}

	logrus.Info("Creating Controller")
	co, err := cassandramulticluster.NewController(clusters, namespace)
	if err != nil {
		log.Fatalf("creating Cassandra Multi Cluster controller: %v", err)
	}

	m := manager.New()
	m.AddController(co)


	logrus.Info("Starting Manager.")
	if err := m.Start(signals.SetupSignalHandler()); err != nil {
		log.Fatalf("while or after starting manager: %v", err)
	}
}
