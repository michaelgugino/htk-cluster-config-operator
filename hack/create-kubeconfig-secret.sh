#!/bin/bash

cat > ./hosted-kubeconfig.secret.yaml <<EOF
apiVersion: v1
kind: Secret
metadata:
  name: hosted-kubeconfig
data:
  value: $(cat $KUBECONFIG | base64 -w0)
EOF
