#!/bin/bash
SCRIPT_DIR=$(dirname $0)
cat > ./hosted-kubecontrolplane.secret.yaml <<EOF
apiVersion: v1
kind: Secret
metadata:
  name: hosted-kubecontrolplane
data:
  kubecontrolplane: $(cat ${SCRIPT_DIR}/kubecontrolplane.config.yaml | base64 -w0)
EOF


cat > ./hosted-oapi.secret.yaml <<EOF
apiVersion: v1
kind: Secret
metadata:
  name: hosted-openshift-apiserver
data:
  config.yaml: $(cat ${SCRIPT_DIR}/ocp.config.yaml | base64 -w0)
EOF
