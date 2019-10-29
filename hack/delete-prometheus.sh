NAMESPACE="kube-system"
BASE_DIR=$(cd $(dirname $0)/..; pwd)
PROMETHEUS_NAMESPACE="my-prometheus"


# Give help text for parameters.
function usage()
{
    echo -e "./delete-deployment.sh "
    echo -e "\t-h --help"
    echo -e "\t--namespace=${NAMESPACE}"
    echo -e "\t Namespace is where the pod was created. Prometheus was by default installed under my-prometheus namespace"
    
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
export NAMESPACE
echo "Delete Prometheus operator"

cat ${BASE_DIR}/deployments/prometheus.yaml | \
	sed -e "s|\${NAMESPACE}|${NAMESPACE}|g" | \
	kubectl -n ${PROMETHEUS_NAMESPACE} delete -f -


cat ${BASE_DIR}/deployments/prometheus-roles.yaml | \
	sed -e "s|\${NAMESPACE}|${NAMESPACE}|g" | \
    sed -e "s|\${PROMETHEUS_NAMESPACE}|${PROMETHEUS_NAMESPACE}|g" | \
	kubectl -n ${NAMESPACE} delete -f -


kubectl delete subs my-prometheus --wait -n ${PROMETHEUS_NAMESPACE} > /dev/null 2>&1
kubectl delete operatorgroup operatorgroup --wait -n $PROMETHEUS_NAMESPACE --all > /dev/null 2>&1
kubectl delete pod,configmap,deployment,secret,sts --wait -n ${PROMETHEUS_NAMESPACE} --all > /dev/null 2>&1
kubectl delete namespace --wait ${PROMETHEUS_NAMESPACE} > /dev/null 2>&1




#