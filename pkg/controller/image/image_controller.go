package image

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"github.com/ghodss/yaml"

	configv1 "github.com/openshift/api/config/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	// metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	//"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	//"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
	//"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	kubecontrolplanev1 "github.com/openshift/api/kubecontrolplane/v1"
	openshiftcontrolplanev1 "github.com/openshift/api/openshiftcontrolplane/v1"

	"github.com/michaelgugino/htk-cluster-config-operator/pkg/util"
)

var log = logf.Log.WithName("controller_image")

// Add creates a new Image Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.  mgmtClient is the client for the
// management cluster, typically the cluster that the operator's pod is running on.
func Add(mgr manager.Manager, mgmtClient client.Client, mgmtNamespace string) error {
	return add(mgr, newReconciler(mgr, mgmtClient, mgmtNamespace))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager, mgmtClient client.Client, mgmtNamespace string) reconcile.Reconciler {
	scheme := mgr.GetScheme()
	codecs := serializer.NewCodecFactory(scheme)
	return &ReconcileImage{
		client: mgr.GetClient(),
		scheme: scheme,
		mgmtClient: mgmtClient,
		mgmtNamespace: mgmtNamespace,
		kdeserializer: codecs.UniversalDecoder(kubecontrolplanev1.SchemeGroupVersion),
		odeserializer: codecs.UniversalDecoder(openshiftcontrolplanev1.SchemeGroupVersion),
	}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("image-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource Image
	err = c.Watch(&source.Kind{Type: &configv1.Image{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileImage implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileImage{}

// ReconcileImage reconciles a Image object
type ReconcileImage struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
	// Performs actions on the management cluster
	mgmtClient client.Client
	// Namespace on the management cluster to look for objects
	mgmtNamespace string
	kdeserializer runtime.Decoder
	odeserializer runtime.Decoder
}

// Reconcile reads that state of the cluster for a Image object and makes changes based on the state read
// and what is in the Image.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileImage) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling Image")
	if request.Name != "cluster" {
		reqLogger.Info("Unknown image config, ignoring")
		return reconcile.Result{}, nil
	}
	// Fetch the Image instance
	configImage := &configv1.Image{}
	err := r.client.Get(context.TODO(), request.NamespacedName, configImage)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	kcpSecret := corev1.Secret{}
	secretKey := client.ObjectKey{
		Namespace: r.mgmtNamespace,
		Name:      util.KcpSecretName,
	}

	if err := r.mgmtClient.Get(context.TODO(), secretKey, &kcpSecret); err != nil {
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	kcp, err := r.kubeCPconfigFromSecret(kcpSecret)

	kcpNew := kcp.DeepCopy()
	//kcpNew := &kubecontrolplanev1.KubeAPIServerConfig{}
	//kcp.DeepCopyInto(kcpNew)


	oapiSecret := corev1.Secret{}
	secretKey = client.ObjectKey{
		Namespace: r.mgmtNamespace,
		Name:      util.OapiSecretName,
	}

	if err := r.mgmtClient.Get(context.TODO(), secretKey, &oapiSecret); err != nil {
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	oapiCP, err := r.oapiConfigFromSecret(oapiSecret)
	oapiCPNew := oapiCP.DeepCopy()

	// internalRegistryHostnamePath := []string{"imagePolicyConfig", "internalRegistryHostname"}
	internalRegistryHostName := configImage.Status.InternalRegistryHostname
	// This should probably never be zero-length.
	if len(internalRegistryHostName) > 0 {
		kcpNew.ImagePolicyConfig.InternalRegistryHostname = internalRegistryHostName
		oapiCPNew.ImagePolicyConfig.InternalRegistryHostname = internalRegistryHostName
	}
	externalRegistryHostnames := configImage.Spec.ExternalRegistryHostnames
	externalRegistryHostnames = append(externalRegistryHostnames, configImage.Status.ExternalRegistryHostnames...)

	kcpNew.ImagePolicyConfig.ExternalRegistryHostnames = externalRegistryHostnames
	oapiCPNew.ImagePolicyConfig.ExternalRegistryHostnames = externalRegistryHostnames

	allowed := configImage.Spec.AllowedRegistriesForImport
	oapiCPNew.ImagePolicyConfig.AllowedRegistriesForImport, err = Convert(allowed)
	if err != nil {
		return reconcile.Result{}, err
	}

	if !reflect.DeepEqual(*kcpNew, kcp) {
		kcpNewSecret := kcpSecret.DeepCopy()
		err = r.updateKCPSecret(kcpNew, kcpNewSecret)
		if err != nil {
			return reconcile.Result{}, err
		}
	}

	if !reflect.DeepEqual(*oapiCPNew, oapiCP) {
		oapiNewSecret := oapiSecret.DeepCopy()
		err = r.updateOAPISecret(oapiCPNew, oapiNewSecret)
		if err != nil {
			return reconcile.Result{}, err
		}
	}

	return reconcile.Result{}, nil
}

func (r *ReconcileImage) updateKCPSecret(kcpNew *kubecontrolplanev1.KubeAPIServerConfig, kcpNewSecret *corev1.Secret) error {
	data, err := yaml.Marshal(*kcpNew)
	if err != nil {
		return err
	}
	kcpNewSecret.Data["kubecontrolplane"] = data
	fmt.Println("updating secret")
	if err := r.client.Update(context.Background(), kcpNewSecret); err != nil {
		fmt.Println("error updating secret")
		return err
	}
	return nil
}

func (r *ReconcileImage) updateOAPISecret(oapiCPNew *openshiftcontrolplanev1.OpenShiftAPIServerConfig, oapiNewSecret *corev1.Secret) error {
	data, err := yaml.Marshal(*oapiCPNew)
	if err != nil {
		return err
	}
	oapiNewSecret.Data["config.yaml"] = data
	fmt.Println("updating secret")
	if err := r.client.Update(context.Background(), oapiNewSecret); err != nil {
		fmt.Println("error updating secret")
		return err
	}
	return nil
}

func (r *ReconcileImage) kubeCPconfigFromSecret(secret corev1.Secret) (kubecontrolplanev1.KubeAPIServerConfig, error) {
	decoded := kubecontrolplanev1.KubeAPIServerConfig{}

	encoded, ok := secret.Data[util.KcpSecretDataField]
	if !ok {
		return decoded, fmt.Errorf("missing key value in secret data")
	}

	if _, _, err := r.kdeserializer.Decode(encoded, nil, &decoded); err != nil {
		fmt.Println("error decoding")
		return decoded, err
	}

	// We can probably use the code below to improve code reuse.
	/*
	configJson, err := yaml.YAMLToJSON(encoded)
	if err != nil {
		fmt.Println("error yaml to json")
		return decoded, err
	}

	cfg := kubecontrolplanev1.KubeAPIServerConfig{}
	if err := json.Unmarshal(configJson, &cfg); err != nil {
		fmt.Println("error json unmarshal")
		return decoded, err
	}
	fmt.Println("json umarshal: ", cfg)

	err := json.Unmarshal(kubecontrolplane, &decoded)
    if err != nil {
        fmt.Println("error:", err)
    }
	*/
	return decoded, nil
}

func (r *ReconcileImage) oapiConfigFromSecret(secret corev1.Secret) (openshiftcontrolplanev1.OpenShiftAPIServerConfig, error) {
	decoded := openshiftcontrolplanev1.OpenShiftAPIServerConfig{}

	encoded, ok := secret.Data[util.OapiSecretDataField]
	if !ok {
		return decoded, fmt.Errorf("missing key config.yaml in secret data")
	}

	if _, _, err := r.odeserializer.Decode(encoded, nil, &decoded); err != nil {
		fmt.Println("error decoding")
		return decoded, err
	}
	return decoded, nil
}

func Convert(o interface{}) (openshiftcontrolplanev1.AllowedRegistries, error) {
	if o == nil {
		return nil, nil
	}
	buf := &bytes.Buffer{}
	if err := json.NewEncoder(buf).Encode(o); err != nil {
		return nil, err
	}

	ret := openshiftcontrolplanev1.AllowedRegistries{}
	if err := json.NewDecoder(buf).Decode(&ret); err != nil {
		return nil, err
	}

	return ret, nil
}
