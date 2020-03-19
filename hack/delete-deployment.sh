NAMESPACE="kube-system"
PROMETHEUS_NAMESPACE="monitoring"
BASE_DIR=$(cd $(dirname $0)/..; pwd)


# Give help text for parameters.
function usage()
{
    echo -e "./delete-deployment.sh "
    echo -e "\t-h --help"
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

kubectl -n ${NAMESPACE} delete -f ${BASE_DIR}/deployments/service.yaml

cat ${BASE_DIR}/deployments/webhook.yaml | \
	${BASE_DIR}/hack/webhook-patch-ca-bundle.sh | \
    sed -e "s|\${NAMESPACE}|${NAMESPACE}|g" | \
	kubectl -n ${NAMESPACE} delete -f -

cat ${BASE_DIR}/deployments/prometheus-roles.yaml | \
	sed -e "s|\${NAMESPACE}|${NAMESPACE}|g" | \
    sed -e "s|\${PROMETHEUS_NAMESPACE}|${PROMETHEUS_NAMESPACE}|g" | \
	kubectl -n ${NAMESPACE} delete -f -

kubectl -n ${NAMESPACE} delete -f ${BASE_DIR}/deployments/deployment.yaml
kubectl -n ${NAMESPACE} delete -f ${BASE_DIR}/deployments/roles.yaml





