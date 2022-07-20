#!/bin/bash

# Always exit on errors.
set -e

# Set our known directories and parameters.
BASE_DIR=$(cd $(dirname $0)/..; pwd)
NAMESPACE="kube-system"
PROMETHEUS_NAMESPACE="monitoring"
OPERATOR_NAMESPACE="operators"
INSTALL_SELF_SIGNED_CERT=true

# Give help text for parameters.
function usage()
{
    echo -e "./webhook-deployment.sh"
    echo -e "\t-h --help"
    echo -e "\t--install-self-signed-cert=${INSTALL_SELF_SIGNED_CERT}"
    echo -e "\t--namespace=${NAMESPACE}"
}
# Parse parameters given as arguments to this script.
while [ "$1" != "" ]; do
    PARAM=`echo $1 | awk -F= '{print $1}'`
    VALUE=`echo $1 | awk -F= '{print $2}'`
    case $PARAM in
        -h | --help)
            usage
            exit
            ;;
        --install-self-signed-cert)
            INSTALL_SELF_SIGNED_CERT=$VALUE
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

if [ "${INSTALL_SELF_SIGNED_CERT}" == true ]; then
	${BASE_DIR}/hack/webhook-create-signed-cert.sh --namespace ${NAMESPACE}
fi

kubectl -n ${NAMESPACE} create -f ${BASE_DIR}/deployments/roles.yaml
kubectl -n ${NAMESPACE} create -f ${BASE_DIR}/deployments/deployment.yaml

kubectl -n ${NAMESPACE} create -f ${BASE_DIR}/deployments/service.yaml
export NAMESPACE
cat ${BASE_DIR}/deployments/webhook.yaml | \
	${BASE_DIR}/hack/webhook-patch-ca-bundle.sh | \
	sed -e "s|\${NAMESPACE}|${NAMESPACE}|g" | \
	kubectl -n ${NAMESPACE} create -f -


sleep 5
if [[ "$(kubectl get pod -l k8s-app=prometheus-operator -n ${OPERATOR_NAMESPACE} | grep -o prometheus-operator)" == "prometheus-operator" ]]; then
cat ${BASE_DIR}/deployments/prometheus-roles.yaml | \
	sed -e "s|\${NAMESPACE}|${NAMESPACE}|g" | \
    sed -e "s|\${PROMETHEUS_NAMESPACE}|${PROMETHEUS_NAMESPACE}|g" | \
	kubectl -n ${NAMESPACE} apply -f -
fi

