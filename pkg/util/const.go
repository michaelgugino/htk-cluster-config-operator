package util

const (
    KcpSecretName = "hosted-kubecontrolplane"
    KcpSecretDataField = "kubecontrolplane"
    KcpDeploymentName = "kube-apiserver"
    OapiSecretName = "hosted-openshift-apiserver"
    OapiSecretDataField = "config.yaml"
    OapiDeploymentName = "openshift-apiserver"
    ConfHashAnnotationName = "x.openshift.io/config-hash"
)
