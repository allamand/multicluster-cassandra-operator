#!/bin/bash

echo ""
echo "Create GKE cluster"
echo ""
./tools/setup-k8s-2-clusters-istio-operator.sh -c  -n cassandra-demo gke_dfy-bac-a-sable_europe-west1-b_istio1 gke_dfy-bac-a-sable_us-central1-a_istio2

echo ""
echo "Configure GKE Firewall"
echo ""

./tools/setup-k8s-2-clusters-istio-operator.sh -f  -n cassandra-demo gke_dfy-bac-a-sable_europe-west1-b_istio1 gke_dfy-bac-a-sable_us-central1-a_istio2


echo ""
echo "Install Istio"
echo ""

./tools/setup-k8s-2-clusters-istio-operator.sh -i  -n cassandra-demo gke_dfy-bac-a-sable_europe-west1-b_istio1 gke_dfy-bac-a-sable_us-central1-a_istio2


echo ""
echo "Install Istio Remote"
echo ""

./tools/setup-k8s-2-clusters-istio-operator.sh -r  -n cassandra-demo gke_dfy-bac-a-sable_europe-west1-b_istio1 gke_dfy-bac-a-sable_us-central1-a_istio2


