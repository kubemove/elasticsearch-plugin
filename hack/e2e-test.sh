#!/usr/bin/env bash
set -eou pipefail

echo "KUBECONFIG: ${KUBECONFIG}"
kubectl config view

#SRC_CONTROL_PANE=$(kubectl get pod -n kube-system  -o name --context=${SRC_CONTEXT}| grep kube-apiserver)
#SRC_CLUSTER_IP=$(kubectl get -n kube-system ${SRC_CONTROL_PANE} -o yaml --context=${SRC_CONTEXT}| grep advertise-address= | cut -c27-)
#
#DST_CONTROL_PANE=$(kubectl get pod -n kube-system  -o name --context=${DST_CONTEXT}| grep kube-apiserver)
#DST_CLUSTER_IP=$(kubectl get -n kube-system ${DST_CONTROL_PANE} -o yaml --context=${DST_CONTEXT}| grep advertise-address= | cut -c27-)
#
#SRC_ES_NODEPORT=$(kubectl get service sample-es-es-http -o yaml --context=${SRC_CONTEXT} | grep nodePort | cut -c15-)
#DST_ES_NODEPORT=$(kubectl get service sample-es-es-http -o yaml --context=${DST_CONTEXT} | grep nodePort | cut -c15-)
#
#SRC_PLUGIN_NODEPORT=$(kubectl get service elasticsearch-plugin -o yaml --context=${SRC_CONTEXT} | grep nodePort | cut -c15-)
#DST_PLUGIN_NODEPORT=$(kubectl get service elasticsearch-plugin -o yaml --context=${DST_CONTEXT} | grep nodePort | cut -c15-)
#
#echo "Running e2e tests:"
#ginkgo -r --v -race --progress --trace ${GINKGO_ARGS} test \
#  --                                                       \
#  --kubeconfigpath=${KUBECONFIG}                           \
#  --src-context=${SRC_CONTEXT}                             \
#  --dst-context=${DST_CONTEXT}                             \
#  --src-plugin=${SRC_CLUSTER_IP}:${SRC_PLUGIN_NODEPORT}    \
#  --dst-plugin=${DST_CLUSTER_IP}:${DST_PLUGIN_NODEPORT}    \
#  --src-cluster-ip=${SRC_CLUSTER_IP}                       \
#  --dst-cluster-ip=${DST_CLUSTER_IP}                       \
#  --src-es-nodeport=${SRC_ES_NODEPORT}                     \
#  --dst-es-nodeport=${DST_ES_NODEPORT}
