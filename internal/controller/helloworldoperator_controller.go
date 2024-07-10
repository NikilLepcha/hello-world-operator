/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"fmt"
	"reflect"

	hwgroupv1 "github.com/NikilLepcha/hello-world-operator/api/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// HelloWorldOperatorReconciler reconciles a HelloWorldOperator object
type HelloWorldOperatorReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=hwgroup.mydomain.io,resources=helloworldoperators,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=hwgroup.mydomain.io,resources=helloworldoperators/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=hwgroup.mydomain.io,resources=helloworldoperators/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the HelloWorldOperator object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.18.2/pkg/reconcile
func (r *HelloWorldOperatorReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the HelloWorldOperator instance
	helloWorld := &hwgroupv1.HelloWorldOperator{}
	err := r.Get(ctx, req.NamespacedName, helloWorld)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("HelloWorldOperator resource not found. Ignoring since object must be deleted.")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get HelloWorldOperator")
		return ctrl.Result{}, err
	}

	// Define the desired ConfigMap object
	configData := fmt.Sprintf(`
global:
  scrape_interval: %s
scrape_configs:
  - job_name: 'prometheus'
    static_configs:
    - targets: ['localhost:9090']
`, helloWorld.Spec.ScrapeInterval)

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "hw-prom-config",
			Namespace: helloWorld.Namespace,
		},
		Data: map[string]string{
			"prometheus.yml": configData,
		},
	}

	// Set HelloWorld instance as the owner and controller
	if err := controllerutil.SetControllerReference(helloWorld, cm, r.Scheme); err != nil {
		logger.Error(err, "Failed to set owner reference on ConfigMap")
		return ctrl.Result{}, err
	}

	// Check if the ConfigMap already exists
	found := &corev1.ConfigMap{}
	err = r.Get(ctx, types.NamespacedName{Name: cm.Name, Namespace: cm.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		logger.Info("Creating a new ConfigMap", "ConfigMap.Namespace", cm.Namespace, "ConfigMap.Name", cm.Name)
		err = r.Create(ctx, cm)
		if err != nil {
			logger.Error(err, "Failed to create new ConfigMap", "ConfigMap.Namespace", cm.Namespace, "ConfigMap.Name", cm.Name)
			return ctrl.Result{}, err
		}
	} else if err != nil {
		logger.Error(err, "Failed to get ConfigMap")
		return ctrl.Result{}, err
	} else {
		logger.Info("ConfigMap already exists", "ConfigMap.Namespace", found.Namespace, "ConfigMap.Name", found.Name)
		// Check if the desired and actual config are the same
		if isEqualConfigMap(cm, found) {
			logger.Info("ConfigMap is already in the desired state", "ConfigMap.Namespace", found.Namespace, "ConfigMap.Name", found.Name)
		} else {
			logger.Info("Updating existing ConfigMap to desired state", "ConfigMap.Namespace", found.Namespace, "ConfigMap.Name", found.Name)
			found.Data = cm.Data
			err = r.Update(ctx, found)
			if err != nil {
				logger.Error(err, "Failed to update ConfigMap", "ConfigMap.Namespace", found.Namespace, "ConfigMap.Name", found.Name)
				return ctrl.Result{}, err
			}
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *HelloWorldOperatorReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&hwgroupv1.HelloWorldOperator{}).
		Owns(&corev1.ConfigMap{}).
		Complete(r)
}

// Helper function to check if two ConfigMaps are equal
func isEqualConfigMap(cm1, cm2 *corev1.ConfigMap) bool {
	return reflect.DeepEqual(cm1.Data, cm2.Data)
}
