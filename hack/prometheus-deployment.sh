#!/bin/bash

# Always exit on errors.
set -e

# Set our known directories and parameters.
BASE_DIR=$(cd $(dirname $0)/..; pwd)
NAMESPACE="kube-system"
PROMETHEUS_NAMESPACE="my-prometheus"


# Give help text for parameters.
function usage()
{
    echo -e "./prometheus-deployment.sh"
    echo -e "\t-h --help"
    echo -e "\t--namespace=${NAMESPACE}"
    echo -e "\t Namespace is where the pod was created. Prometheus will install under my-prometheus namespace"
    
}


# Parse parameters given as arguments to this script.

# Parse parameters given as arguments to this script.
while [ "$1" != "" ]; do
    PARAM=`echo $1 | awk -F= '{print $1}'`
    VALUE=`echo $1 | awk -F= '{print $2}'`
    case $PARAM in
        -h | --help)
            usage
            exit
            ;;
	--namespace)
            NAMESPACE=$VALUE
            ;;
	*)
            echo "ERROR: unknown parameter \"$PARAM\""
            usage
            exit 1
            ;;
    esac
    shift
done
export NAMESPACE
echo "Creating OlLM package"
if [[ "$(kubectl api-resources | grep -o operatorgroup)" != "operatorgroup" ]]; then
curl -sL https://github.com/operator-framework/operator-lifecycle-manager/releases/download/0.12.0/install.sh | bash -s 0.12.0
sleep 5
else
echo "olm is already installed, skipping installation."
fi 

echo "Deploying Prometheus operator"
kubectl create -f https://operatorhub.io/install/prometheus.yaml
echo "Check that clusterserviceversion status is set to \"Succeeded\""
  while ! [[ `kubectl get csv --namespace=${PROMETHEUS_NAMESPACE} -o json | jq .items[0].status.phase` == \"Succeeded\" ]]; do
      sleep 6
      retries=$((retries - 1))
      if [[ $retries == 0 ]]; then
        >&2 echo "failed to reach "Succeeded" clusterserviceversion status for \"$csv\""
        # Print out CSV's 'status' section on error
        kubectl get csv --namespace=${PROMETHEUS_NAMESPACE}  -o jsonpath="{.status}"
        exit 1
      fi
      echo "clusterserviceversion status : `kubectl get csv --namespace=${PROMETHEUS_NAMESPACE}  -o json | jq .items[0].status.phase`"
  done


cat ${BASE_DIR}/deployments/prometheus.yaml | \
	sed -e "s|\${NAMESPACE}|${NAMESPACE}|g" | \
	kubectl -n ${PROMETHEUS_NAMESPACE} create -f -


cat ${BASE_DIR}/deployments/prometheus-roles.yaml | \
	sed -e "s|\${NAMESPACE}|${NAMESPACE}|g" | \
    sed -e "s|\${PROMETHEUS_NAMESPACE}|${PROMETHEUS_NAMESPACE}|g" | \
	kubectl -n ${NAMESPACE} create -f -

