#!/bin/bash

rm -rf "./deploy"
mkdir  "./deploy"  && touch "./deploy/workshop_operator.yaml"

OUTPUT_YAML="./deploy/workshop_operator.yaml"
NAMESPACE="workshop-infra"

kubectl create namespace $NAMESPACE --dry-run -o yaml >> $OUTPUT_YAML

CRD_LOC="./charts/workshop-operator/crds"
echo $CRD_LOC

for filename in $CRD_LOC/workshop*.yaml; do
  echo $filename
  cat $filename >> $OUTPUT_YAML
done

helm template --namespace $NAMESPACE workshop-operator ./charts/workshop-operator >> $OUTPUT_YAML

cat $OUTPUT_YAML