#!/bin/bash

rm -rf "./deploy"
mkdir  "./deploy"  && touch "./deploy/workshop_operator.yaml"

OUTPUT_YAML="./deploy/workshop_operator.yaml"
NAMESPACE="workshop-infra"
LABELNAME="control-plane"
LABELVALUE="controller-manager"

curl -L -o yq_linux.tar.gz https://github.com/mikefarah/yq/releases/download/v4.12.2/yq_linux_amd64.tar.gz &&\
 tar -xzvf yq_linux.tar.gz && \
 sudo mv yq_linux_amd64 /usr/local/bin/yq && \
 rm yq_linux.tar.gz

kubectl create namespace $NAMESPACE && kubectl label namespace $NAMESPACE $LABELNAME=$LABELVALUE  --dry-run -o yaml  | yq eval 'del(.metadata.creationTimestamp, .metadata.managedFields, .metadata.resourceVersion, .metadata.selfLink, .metadata.selfLink, .metadata.uid, .spec, .status)' - >> $OUTPUT_YAML

CRD_LOC="./charts/workshop-operator/crds"
echo $CRD_LOC

for filename in $CRD_LOC/workshop*.yaml; do
  echo $filename
  cat $filename >> $OUTPUT_YAML
done

helm template --namespace $NAMESPACE workshop-operator ./charts/workshop-operator >> $OUTPUT_YAML

cat $OUTPUT_YAML