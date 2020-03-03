#!/usr/bin/env bash
set -eou pipefail

echo "Deleting sample Easticsearch from the source cluster...."
kubectl delete -f ./deploy/elasticsearch.yaml --context=${SRC_CONTEXT} || true
echo "Deleting sample Easticsearch from the destination cluster...."
kubectl delete -f ./deploy/elasticsearch.yaml --context=${DST_CONTEXT} || true

echo " "
echo "Uninstalling Elasticsearch Plugin from the source cluster...."
kubectl delete -f ./deploy/plugin.yaml --context=${SRC_CONTEXT} || true
echo "Uninstalling Elasticsearch Plugin from the destination cluster...."
kubectl delete -f ./deploy/plugin.yaml --context=${DST_CONTEXT} || true

echo ""
echo "Deleting MoveEngine CR from the source cluster...."
kubectl delete -f ./deploy/moveengine.yaml --context=${SRC_CONTEXT} || true
echo "Deleting MoveEngine CR from the destination cluster...."
kubectl delete -f ./deploy/moveengine.yaml --context=${DST_CONTEXT} || true


echo "Uninstalling ECK operator from the source cluster"
kubectl delete -f https://download.elastic.co/downloads/eck/1.0.1/all-in-one.yaml --context=${SRC_CONTEXT} || true
echo "Uninstalling ECK operator from the destination cluster"
kubectl delete -f https://download.elastic.co/downloads/eck/1.0.1/all-in-one.yaml --context=${DST_CONTEXT} || true

echo ""
echo "Registering Kubemove CRDs from the source cluster"
kubectl delete -f ./deploy/dependencies/crds/ --context=${SRC_CONTEXT} || true
echo "Registering Kubemove CRDs from the destination cluster"
kubectl delete -f ./deploy/dependencies/crds/ --context=${DST_CONTEXT} || true

echo ""
echo "Deleting RBAC resources from the source cluster"
kubectl delete -f ./deploy/dependencies/rbac.yaml --context=${SRC_CONTEXT} || true
echo "Deleting RBAC resources from the destination cluster"
kubectl delete -f ./deploy/dependencies/rbac.yaml --context=${DST_CONTEXT} || true

echo ""
echo "Uninstalling DataSync controller from the source cluster"
kubectl delete -f ./deploy/dependencies/datasync_controller.yaml --context=${SRC_CONTEXT} || true
echo "Uninstalling DataSync controller from the destination cluster"
kubectl delete -f ./deploy/dependencies/datasync_controller.yaml --context=${DST_CONTEXT} || true

echo ""
echo "Uninstalling MoveEngine controller from the source cluster"
kubectl delete -f ./deploy/dependencies/moveengine_controller.yaml --context=${SRC_CONTEXT} || true
echo "Uninstalling MoveEngine controller from the destination cluster"
kubectl delete -f ./deploy/dependencies/moveengine_controller.yaml --context=${DST_CONTEXT} || true

echo ""
echo "Deleting Minio Secret from the source cluster"
kubectl delete -f ./deploy/dependencies/minio_secret.yaml --context=${SRC_CONTEXT} || true
echo "Deleting Minio Secret from the destination cluster"
kubectl delete -f ./deploy/dependencies/minio_secret.yaml --context=${DST_CONTEXT} || true

echo ""
echo "Deleting Minio Server from the destination cluster"
kubectl delete -f ./deploy/dependencies/minio_server.yaml --context=${DST_CONTEXT} || true
