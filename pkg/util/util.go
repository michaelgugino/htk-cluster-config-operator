package util

import (
    "fmt"
    "context"
    corev1 "k8s.io/api/core/v1"
    "sigs.k8s.io/controller-runtime/pkg/client"
    "k8s.io/client-go/tools/clientcmd"
    restclient "k8s.io/client-go/rest"
)

func RestConfigFromSecret(c client.Client, localNamespace string) (*restclient.Config, error) {

    secret := &corev1.Secret{}
	secretKey := client.ObjectKey{
		Namespace: localNamespace,
		Name:      "hosted-kubeconfig",
	}

	if err := c.Get(context.TODO(), secretKey, secret); err != nil {
		return nil, err
	}

    kubeconfig, ok := secret.Data["value"]
    if !ok {
        return nil, fmt.Errorf("missing key value in secret data")
    }

    restConfig, err := clientcmd.RESTConfigFromKubeConfig(kubeconfig)
    if err != nil {
        return nil, fmt.Errorf("Failed to get remote restconfig from kubeconfig")
    }
    return restConfig, nil
}
