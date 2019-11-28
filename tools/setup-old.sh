#!/bin/bash


echo "Setup CassandraMulticluster on empty k8s clusters"

OPTIND=1         # Reset in case getopts has been used previously in the shell.

verbose=0
namespace=cassandra-demo
user=sebastien.allamand@orange.com

createcluster=false
createFw=false
installIstio=false
while getopts "h?vn:u:fcir" opt; do
    case "$opt" in
        h|\?)
            show_help
            exit 0
            ;;
        v)  verbose=1
            ;;
        f)  createFw=true
            ;;
        i)  installIstio=true
            ;;
        r)  installRemoteIstio=true
            ;;
        c)  createcluster=true
            ;;
        u)  user=$OPTARG
            ;;
        n)  namespace=$OPTARG
            ;;
    esac
done

shift $((OPTIND-1))
#[ "${1:-}" = "--" ] && shift

if [[ $# -lt 1 ]]; then
    echo "usage: cluser1 cluster2 clustern"
    exit 0
fi

set -x
if [ "$createcluster" = true ]; then
echo "configuring GKE clusters $@"

for cluster in $@; do
    echo -e "\nconfiguring cluster $cluster $namespace $user"

    kubectl config use-context $cluster

    echo -e "  $cluster: configuring helm"
    helm --kube-context $cluster init
    kubectl --context $cluster create clusterrolebinding cluster-admin-binding --clusterrole cluster-admin --user "$(gcloud config get-value core/account)"
    kubectl  --context $cluster create serviceaccount --namespace kube-system tiller
    kubectl  --context $cluster create clusterrolebinding tiller-cluster-rule --clusterrole=cluster-admin --serviceaccount=kube-system:tiller
    kubectl  --context $cluster patch deploy --namespace kube-system tiller-deploy -p '{"spec":{"template":{"spec":{"serviceAccount":"tiller"}}}}'

    echo "  $cluster: deploying CRDs"
    kubectl --context $cluster create namespace $namespace
    kubectl --context $cluster --namespace $namespace apply -f https://raw.githubusercontent.com/Orange-OpenSource/cassandra-k8s-operator/master/deploy/crds/db_v1alpha1_cassandracluster_crd.yaml
    kubectl --context $cluster --namespace $namespace apply -f deploy/crds/multicluster_v1alpha1_cassandramulticluster_crd.yaml

    echo "  $cluster: deploying CassKop"
    kubens $namespace
    #don't see how to deploy on target namespace in command line..
    helm  --kube-context $cluster install --name casskop casskop/cassandra-operator --no-hooks
done
fi




if [ "$createFw" = true ]; then
    echo "Create GoogleCloud Firewall rule"
    function join_by { local IFS="$1"; shift; echo "$*"; }
    ALL_CLUSTER_CIDRS=$(gcloud container clusters list --format='value(clusterIpv4Cidr)' | sort | uniq)
    ALL_CLUSTER_CIDRS=$(join_by , $(echo "${ALL_CLUSTER_CIDRS}"))
    ALL_CLUSTER_NETTAGS=$(gcloud compute instances list --format='value(tags.items.[0])' | sort | uniq)
    ALL_CLUSTER_NETTAGS=$(join_by , $(echo "${ALL_CLUSTER_NETTAGS}"))
    gcloud compute firewall-rules create istio-multicluster-test-pods \
       --allow=tcp,udp,icmp,esp,ah,sctp \
       --direction=INGRESS \
       --priority=900 \
       --source-ranges="${ALL_CLUSTER_CIDRS}" \
       --target-tags="${ALL_CLUSTER_NETTAGS}" --quiet

fi

ISTIOPATH=~/gomac/src/github.com/istio/istio
if [ "$installIstio" = true ]; then
    #$1 is the first kubernetes cluster to install istio control plan on
    kubectl config use-context $1
    kubectl apply -f $ISTIOPATH/install/kubernetes/helm/istio-init/files/
    helm template --set kiali.enabled=true $ISTIOPATH/install/kubernetes/helm/istio --name istio --namespace istio-system > $HOME/istio_master.yaml
    kubectl create ns istio-system
    kubectl apply -f $HOME/istio_master.yaml
    kubectl label namespace cassandra-demo istio-injection=enabled

fi

#option -r (remote)
if [ "$installRemoteIstio" = true ]; then
    export PILOT_POD_IP=$(kubectl -n istio-system get pod -l istio=pilot -o jsonpath='{.items[0].status.podIP}')
    export POLICY_POD_IP=$(kubectl -n istio-system get pod -l istio=mixer -o jsonpath='{.items[0].status.podIP}')
    export TELEMETRY_POD_IP=$(kubectl -n istio-system get pod -l istio-mixer-type=telemetry -o jsonpath='{.items[0].status.podIP}')

    helm template $ISTIOPATH/install/kubernetes/helm/istio \
         --namespace istio-system --name istio-remote \
         --values $ISTIOPATH/install/kubernetes/helm/istio/values-istio-remote.yaml \
         --set kiali.enabled=true \
         --set global.remotePilotAddress=${PILOT_POD_IP} \
         --set global.remotePolicyAddress=${POLICY_POD_IP} \
         --set global.remoteTelemetryAddress=${TELEMETRY_POD_IP} > $HOME/istio-remote.yaml

    #$2 is the second kubernetes cluster to install istio control plan on
    kubectl config use-context $2
    kubectl create ns istio-system
    kubectl apply -f $HOME/istio-remote.yaml
    kubectl label namespace cassandra-demo istio-injection=enabled


    # wait to the above to be ready ?

    export WORK_DIR=$(pwd)
    CLUSTER_NAME=$(kubectl config view --minify=true -o jsonpath='{.clusters[].name}')
    CLUSTER_NAME="${CLUSTER_NAME##*_}"
    export KUBECFG_FILE=${WORK_DIR}/${CLUSTER_NAME}
    SERVER=$(kubectl config view --minify=true -o jsonpath='{.clusters[].cluster.server}')
    NAMESPACE=istio-system
    SERVICE_ACCOUNT=istio-multi
    SECRET_NAME=$(kubectl get sa ${SERVICE_ACCOUNT} -n ${NAMESPACE} -o jsonpath='{.secrets[].name}')
    CA_DATA=$(kubectl get secret ${SECRET_NAME} -n ${NAMESPACE} -o jsonpath="{.data['ca\.crt']}")
    TOKEN=$(kubectl get secret ${SECRET_NAME} -n ${NAMESPACE} -o jsonpath="{.data['token']}" | base64 --decode)


    cat <<EOF > ${KUBECFG_FILE}
apiVersion: v1
clusters:
   - cluster:
       certificate-authority-data: ${CA_DATA}
       server: ${SERVER}
     name: ${CLUSTER_NAME}
contexts:
   - context:
       cluster: ${CLUSTER_NAME}
       user: ${CLUSTER_NAME}
     name: ${CLUSTER_NAME}
current-context: ${CLUSTER_NAME}
kind: Config
preferences: {}
users:
   - name: ${CLUSTER_NAME}
     user:
       token: ${TOKEN}
EOF


    #    Configure Istio control plane to discover the remote cluster-2 from cluster-1
    kubectl config use-context $1
    kubectl create secret generic ${CLUSTER_NAME} --from-file ${KUBECFG_FILE} -n ${NAMESPACE}
    kubectl label secret ${CLUSTER_NAME} istio/multiCluster=true -n ${NAMESPACE}


fi

bookinfo=false
if [ "$bookinfo" = true ]; then
    kubectl config use-context $1
    kubectl apply -f $ISTIOPATH/samples/bookinfo/platform/kube/bookinfo.yaml
    kubectl apply -f $ISTIOPATH/samples/bookinfo/networking/bookinfo-gateway.yaml
    kubectl delete deployment reviews-v3

    #Create review service on remote
    kubectl config use-context $2
    kubectl apply -n $namespace -f tools/review-v3.yaml

    #get the istio-ingressgateway service external ip to access bookinfo
    kubectl config use-context $1
    kubectl get svc istio-ingressgateway -n istio-system
fi


cleanup=false
if [ "$cleanup" = true ]; then
    gcloud compute firewall-rules delete istio-multicluster-test-pods --quiet
    kubectl delete clusterrolebinding gke-cluster-admin-binding
    #gcloud container clusters delete cluster-2 --zone $zone

fi
