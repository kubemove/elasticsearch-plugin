#!/usr/bin/env bash
set -eou pipefail

echo "Installing ECK operator in the source cluster"
kubectl apply -f https://download.elastic.co/downloads/eck/1.0.1/all-in-one.yaml --context=${SRC_CONTEXT}
echo "Installing ECK operator in the destination cluster"
kubectl apply -f https://download.elastic.co/downloads/eck/1.0.1/all-in-one.yaml --context=${DST_CONTEXT}

echo ""
echo "Registering Kubemove CRDs in the source cluster"
kubectl apply -f deploy/dependencies/crds/ --context=${SRC_CONTEXT}
echo "Registering Kubemove CRDs in the destination cluster"
kubectl apply -f deploy/dependencies/crds/ --context=${DST_CONTEXT}

echo ""
echo "Creating RBAC resources in the source cluster"
kubectl apply -f deploy/dependencies/rbac.yaml --context=${SRC_CONTEXT}
echo "Creating RBAC resources in the destination cluster"
kubectl apply -f deploy/dependencies/rbac.yaml --context=${DST_CONTEXT}

echo ""
echo "Installing DataSync controller in the source cluster"
kubectl apply -f deploy/dependencies/datasync_controller.yaml --context=${SRC_CONTEXT}
echo "Installing DataSync controller in the destination cluster"
kubectl apply -f deploy/dependencies/datasync_controller.yaml --context=${DST_CONTEXT}

echo ""
echo "Installing MoveEngine controller in the source cluster"
kubectl apply -f deploy/dependencies/moveengine_controller.yaml --context=${SRC_CONTEXT}
echo "Installing MoveEngine controller in the destination cluster"
kubectl apply -f deploy/dependencies/moveengine_controller.yaml --context=${DST_CONTEXT}

echo ""
echo "Creating Minio Secret in the source cluster"
cat ./deploy/dependencies/minio_secret.yaml | envsubst | kubectl apply -f - --context=${SRC_CONTEXT}
echo "Creating Minio Secret in the destination cluster"
cat ./deploy/dependencies/minio_secret.yaml | envsubst | kubectl apply -f - --context=${DST_CONTEXT}

echo ""
echo "Deploying Minio Server in the destination cluster"
kubectl apply -f ./deploy/dependencies/minio_server.yaml --context=${DST_CONTEXT}

echo ""
echo "Waiting for all pods of of the source cluster to be ready"
kubectl wait --for=condition=READY pods --all --timeout=5m --context=${SRC_CONTEXT}
echo "Waiting for all pods of of the source cluster to be ready"
kubectl wait --for=condition=READY pods --all --timeout=5m --context=${DST_CONTEXT}

MINIO_NODEPORT:=$(kubectl get service minio -o yaml --context=${DST_CONTEXT} | grep nodePort | cut -c15-)
MINIO_SERVER_ADDRESS:=${DST_CLUSTER_IP}:${MINIO_NODEPORT}

echo "Creating demo bucket in the  Minio server"
mc config host add es-repo http://${MINIO_SERVER_ADDRESS} ${MINIO_ACCESS_KEY} ${MINIO_SECRET_KEY}
mc mb es-repo/demo
