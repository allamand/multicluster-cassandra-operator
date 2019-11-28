#!/bin/bash

################################################################################
# Default Configuration
################################################################################

casskopHelmRelease=casskop
casskopHelmVersion=0.3.1
casskopHelmRepo=casskop/cassandra-operator
casskopDockerRepo=ext-dockerio.artifactory.si.francetelecom.fr/orangeopensource/cassandra-k8s-operator
namespace=cassandra-demo

################################################################################

echo "Setup CassandraMulticluster on empty k8s clusters"

OPTIND=1         # Reset in case getopts has been used previously in the shell.
verbose=0

createCluster=false
installCassKop=false
createFw=false
installIstio=false
simpleTest=false

usage() {
    echo "usage: setup.sh [opts] [action] [action-opts] cluser1 cluster2 clustern"
    echo "actions = casskop|firewall|istio|istio-remote|simple-test"
    echo "action-opts: -n namespace|-r helm-release|-v helm-version "
    echo "ex:  ./tools/setup-casskop.sh casskop -n cassandra-seb -r casskop-seb -v 0.3.1 dex-sallamand-kaas-prod-priv-sph dex-sallamand-kaas-prod-priv-bgl"
    exit 0
    
}

# Parse options to the `pip` command
while getopts ":hn:" opt; do
    case ${opt} in
        h )
            usage
            ;;
        n)  namespace=$OPTARG
            ;;
        \? )
            echo "Invalid Option: -$OPTARG" 1>&2
            exit 1
            ;;
    esac
done
shift $((OPTIND -1))
subcommand=$1; shift  # Remove 'pip' from the argument list
case "$subcommand" in
  # Parse options to the install sub command
    casskop)
        installCassKop=true
        shift $((OPTIND -1))
        #process casskop args
        while getopts ":n:r:v:" opt; do
            echo ${opt} $OPTARG
            case ${opt} in
                n)  namespace=$OPTARG
                    ;;
                r)  casskopHelmRelease=$OPTARG
                    ;;
                v)  casskopHelmVersion=$OPTARG
                    ;;               
                \? )
                    echo "Invalid Option: -$OPTARG" 1>&2
                    echo "usage: setup casskop [-n namespace] site1 site2..siteN"
                    exit 1
                    ;;
                : )
                    echo "Invalid Option: -$OPTARG requires an argument" 1>&2
                    exit 1
                    ;;
            esac
        done
        shift $((OPTIND-1))

        ;;
    firewall)
        createFw=true
        shift $((OPTIND -1))
        ;;
    istio)
        installIstio=true
        shift $((OPTIND -1))
        ;;
    istio-remote)
        installRemoteIstio=true
        shift $((OPTIND -1))
        ;;
    simple-test)
        simpleTest=true
        shift $((OPTIND -1))
        ;;
esac


if [[ $# -lt 1 ]]; then
    usage
fi






#gke_dfy-bac-a-sable_europe-west1-b_cluster-1
contexts=$@
i=0
for context in $contexts; do
    reste=${context%_*}
    contexts[$i]=$context
    clusters[$i]=${context##*_}
    zones[$i]=${reste##*_}
    i=$i+1
done

echo "subcommand $subcommand"
echo "namespace $namespace"
echo ${#contexts[*]} "contexts: " ${contexts[@]}
echo ${#clusters[*]} "clusters: " ${clusters[@]}
echo ${#zones[*]} "zones: " ${zones[*]}



if [$GKE]; then
    if [ "$createCluster" = true ]; then
        for ((i=0; i<${#clusters[*]}; i++)); do
            set -x
            gcloud container clusters create  ${clusters[i]} --enable-ip-alias --zone  ${zones[i]} --machine-type n1-standard-4 --num-nodes=4 --preemptible --async --enable-network-policy
            set +x
        done

        while gcloud container clusters list --format='value(Status)' | grep -v RUNNING > /dev/null; do
            echo -n "."
            sleep 20
        done
        
        for ((i=0; i<${#clusters[*]}; i++)); do
            gcloud container clusters get-credentials  ${clusters[i]} --zone  ${zones[i]}
            context=${contexts[i]}
            echo -e "\nconfiguring context $context $namespace"
            
            set -x
            kubectl config use-context $context
            
            echo -e "  $context: configuring helm"
            helm --kube-context $context init
            kubectl --context $context create clusterrolebinding cluster-admin-binding --clusterrole cluster-admin --user "$(gcloud config get-value core/account)"
            kubectl  --context $context create serviceaccount --namespace kube-system tiller
            kubectl  --context $context create clusterrolebinding tiller-cluster-rule --clusterrole=cluster-admin --serviceaccount=kube-system:tiller
            kubectl  --context $context patch deploy --namespace kube-system tiller-deploy -p '{"spec":{"template":{"spec":{"serviceAccount":"tiller"}}}}'
            
            kubectl delete storageclasses.storage.k8s.io standard
            kubectl apply -f https://raw.githubusercontent.com/Orange-OpenSource/cassandra-k8s-operator/master/samples/gke-storage-standard-wait.yaml
            
        done
    fi
fi

#casskop
if [ "$installCassKop" = true ]; then
    for ((i=0; i<${#contexts[*]}; i++)) do
        context=${contexts[i]}
        echo "Install CassKop in ${clusters[i]}"

        set -x
        echo "  $context: deploying CRDs"

        echo "create namespace"
        if [[ ${contexts[i]} =~ "sph" ]]; then
            ipPool=routable-sph
        else
            ipPool=routable-bgl
        fi
        cat <<EOF | kubectl --context $context apply -f -
apiVersion: v1
kind: Namespace
metadata:
  name: $namespace
  annotations:
    "cni.projectcalico.org/ipv4pools": "[\"$ipPool\"]"
  labels:
    app: cassandra
EOF

        #kubens $namespace

        echo "deploy multicluster CRD"
        kubectl --context $context --namespace $namespace apply -f deploy/crds/multicluster_v1alpha1_cassandramulticluster_crd.yaml

        echo "apply PSP"
        kubectl --context $context --namespace $namespace apply -f https://raw.githubusercontent.com/Orange-OpenSource/cassandra-k8s-operator/master/deploy/psp-cassie.yaml
        kubectl --context $context --namespace $namespace apply -f https://raw.githubusercontent.com/Orange-OpenSource/cassandra-k8s-operator/master/deploy/psp-sa-cassie.yaml
        kubectl --context $context --namespace $namespace apply -f https://raw.githubusercontent.com/Orange-OpenSource/cassandra-k8s-operator/master/deploy/clusterRole-cassie.yaml


        echo "  $context: deploying CassKop"
        helm --kube-context $context delete --purge $casskopHelmRelease

        #deploy with CRD or withotu if already exists        
        helm  --kube-context $context --namespace $namespace install --name $casskopHelmRelease $casskopHelmRepo --version $casskopHelmVersion --set image.repository=$casskopDockerRepo || \
            helm  --kube-context $context --namespace $namespace install --name $casskopHelmRelease $casskopHelmRepo --version $casskopHelmVersion --set image.repository=$casskopDockerRepo --no-hooks

        echo "install network policies"
        kubectl --context $context --namespace $namespace apply -f $GOPATH/src/gitlab.si.francetelecom.fr/kubernetes/casskop/samples/casskop-net.yaml
        echo "install default configmap"
        kubectl --context $context --namespace $namespace apply -f $GOPATH/src/gitlab.si.francetelecom.fr/kubernetes/casskop/samples/cassandra-configmap-v1.yaml

        set +x
    done
fi






#-f / --firewall
if [ "$createFw" = true ]; then
    echo "Create GoogleCloud Firewall rule"
    clustersComa=""
    clustersPipe=""
    for ((i=0; i<${#clusters[*]}; i++)) do
        cluster=${clusters[$i]}
        clustersComa="$clustersComa,$cluster"
        clustersPipe="$clustersPipe|$cluster"
    done
    clustersComa=${clustersComa:1}
    clustersPipe=${clustersPipe#"|"}

    function join_by { local IFS="$1"; shift; echo "$*"; }
    ALL_CLUSTER_CIDRS=$(gcloud container clusters list --filter "name=($clustersComa)" --format='value(clusterIpv4Cidr)' | sort | uniq)
    ALL_CLUSTER_CIDRS=$(join_by , $(echo "${ALL_CLUSTER_CIDRS}"))
    ALL_CLUSTER_NETTAGS=$(gcloud compute instances list --filter "name ~ $clustersPipe" --format='value(tags.items.[0])' | sort | uniq)
    ALL_CLUSTER_NETTAGS=$(join_by , $(echo "${ALL_CLUSTER_NETTAGS}"))

    set -x
    yes | gcloud compute firewall-rules delete istio-multicluster-remote-test --quiet
    gcloud compute firewall-rules create istio-multicluster-remote-test \
           --allow=tcp,udp,icmp,esp,ah,sctp \
           --direction=INGRESS \
           --priority=900 \
           --source-ranges="${ALL_CLUSTER_CIDRS}" \
           --target-tags="${ALL_CLUSTER_NETTAGS}" --quiet
    set +x
fi




ISTIOPATH=$GOPATH/src/github.com/banzaicloud/istio-operator

#-i / --istio
if [ "$installIstio" = true ]; then

    set -x
    kubectl config use-context ${contexts[0]}
    cd $ISTIOPATH
    make deploy
    #helm repo add banzaicloud-stable https://kubernetes-charts.banzaicloud.com/
    #helm install --name=istio-operator --namespace=istio-system banzaicloud-stable/istio-operator
    cd -
    kubectl create -n istio-system -f samples/istio/istio_v1beta1_istio.yaml
    set +x

fi


#-r / --istio-remote
if [ "$installRemoteIstio" = true ]; then

#    for ((i=1; i<${#contexts[*]}; i++)) do
        context=${contexts[1]}
        echo -e "\nconfiguring context $context $namespace"

        cd $ISTIOPATH

        set -x
        #Change the context to the remote cluster and create the istio-system namespace
        kubectl config use-context $context
        kubectl create namespace istio-system

        #Create a service account and generate kubeconfig for the operator to be able to deploy resources to the remote cluster
        kubectl create -f docs/federation/flat/rbac.yml
        REMOTE_KUBECONFIG_FILE=$(docs/federation/flat/generate-kubeconfig.sh)

        #The kubeconfig for the remote cluster must be added to the central cluster as a secret
        kubectl config use-context ${contexts[0]}
        kubectl create secret generic remoteistio-sample --from-file ${REMOTE_KUBECONFIG_FILE} -n istio-system
        rm -f ${REMOTE_KUBECONFIG_FILE}


        #The added secret must be labeled for Istio
        kubectl label secret remoteistio-sample istio/multiCluster=true -n istio-system

        #Create the Istio remote config on the central cluster and label the default namespace for auto sidecar injection on the remote cluster as well
        #kubectl create -n istio-system -f config/samples/istio_v1beta1_remoteistio.yaml
        cd -
        kubectl create -n istio-system -f samples/istio/istio_v1beta1_remoteistio.yaml


        #Add a simple test echo-service onto both clusters


        set +x
#    done


fi


# -t
if [ "$simpleTest" = true ]; then
    cd $ISTIOPATH
    kubectl config use-context  ${contexts[0]}
    kubectl delete -f docs/federation/flat/echo-service.yml
    kubectl apply -f docs/federation/flat/echo-service.yml

    kubectl config use-context  ${contexts[1]}
    kubectl delete -f docs/federation/flat/echo-service.yml
    kubectl apply -f docs/federation/flat/echo-service.yml

    kubectl config use-context  ${contexts[0]}

    sleep 30
    kubectl -n default exec $(kubectl get pods -n default -l k8s-app=echo -o jsonpath={.items..metadata.name}) -c echo-service -ti -- sh -c 'for i in `seq 1 50`; do curl -s echo | grep -i hostname | cut -d " " -f 2; done | sort | uniq -c'
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
    gcloud compute firewall-rules delete istio-multicluster-remote-test
    #gcloud compute firewall-rules delete istio-multicluster-test-pods --quiet
    kubectl delete clusterrolebinding gke-cluster-admin-binding
    #gcloud container clusters delete cluster-2 --zone $zone

fi
