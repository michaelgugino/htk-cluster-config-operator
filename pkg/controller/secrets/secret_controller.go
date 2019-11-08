package secrets

import (
    "context"
    "crypto/md5"
    "fmt"
    "reflect"

    appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
    apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

    "github.com/michaelgugino/htk-cluster-config-operator/pkg/util"
)

var log = logf.Log.WithName("controller_secret")

// Add creates a new Image Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileSecret{
		client: mgr.GetClient(),
		scheme: mgr.GetScheme(),
	}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("secret-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource Image
	err = c.Watch(&source.Kind{Type: &corev1.Secret{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileSecret implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileSecret{}

// ReconcileSecret reconciles a Image object
type ReconcileSecret struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileSecret) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling Secret")
    // Fetch the Machine instance
	s := &corev1.Secret{}
	if err := r.client.Get(context.TODO(), request.NamespacedName, s); err != nil {
		if apierrors.IsNotFound(err) {
            // This should probably never happen, but there's nothing this controller
            // can do about it.
			return reconcile.Result{}, nil
		}

		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}
    // TODO(michaelgugino): Handle deleted secret here.

	if request.Name == util.KcpSecretName {
		//reqLogger.Info("Unknown image config, ignoring")
        return r.updateDeploymentSecret(s, util.KcpSecretDataField, util.KcpDeploymentName)
        //r.processKube(s)
		// return reconcile.Result{}, nil
	} else if request.Name == util.OapiSecretName {
        return r.updateDeploymentSecret(s, util.OapiSecretDataField, util.OapiDeploymentName)
        //r.processOpenshift(s)
        // return reconcile.Result{}, nil
    }
    reqLogger.Info("Unknown secret, ignoring")
    return reconcile.Result{}, nil
}

func (r *ReconcileSecret) updateDeploymentSecret(s *corev1.Secret, secretFieldName string, deploymentName string) (reconcile.Result, error) {
    encoded, ok := s.Data[secretFieldName]
	if !ok {
		return reconcile.Result{}, fmt.Errorf("missing key %v in secret data", secretFieldName)
	}
    confHash := fmt.Sprintf("%x", md5.Sum(encoded))
    fmt.Println("hash:", confHash)

    d := &appsv1.Deployment{}
    dK := client.ObjectKey{
		Namespace: s.Namespace,
		Name:      deploymentName,
	}
    if err := r.client.Get(context.TODO(), dK, d); err != nil {
		// Error reading the object - requeue the request.
        fmt.Println("couldn't get deployment", dK)
		return reconcile.Result{}, err
	}
    dNew := d.DeepCopy()
    dNew.ObjectMeta.Annotations[util.ConfHashAnnotationName] = confHash
    if dNew.Spec.Template.ObjectMeta.Annotations == nil {
        annotations := make(map[string]string)
        dNew.Spec.Template.ObjectMeta.SetAnnotations(annotations)
    }
    dNew.Spec.Template.ObjectMeta.Annotations[util.ConfHashAnnotationName] = confHash
    if !reflect.DeepEqual(*dNew, *d) {
        if err := r.client.Update(context.TODO(), dNew); err != nil {
            fmt.Println("error updating deployment", deploymentName)
            return reconcile.Result{}, err
        }
        fmt.Println("deployment updated", dK)
	} else {
        fmt.Println("Nothing to update")
    }
    return reconcile.Result{}, nil
}
