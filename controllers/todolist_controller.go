/*
Copyright 2022.

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

package controllers

import (
	"context"
	"fmt"
	"os"

	ocpAppsV1 "github.com/openshift/api/apps/v1"
	applicationv1alpha1 "github.com/workshop/todo-list-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	yamlutil "k8s.io/apimachinery/pkg/util/yaml"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// TodoListReconciler reconciles a TodoList object
type TodoListReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=application.workshop.com,resources=todolists,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=application.workshop.com,resources=todolists/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=application.workshop.com,resources=todolists/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the TodoList object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.10.0/pkg/reconcile
func (r *TodoListReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	Log := log.FromContext(ctx)

	todo := &applicationv1alpha1.TodoList{}
	err := r.Client.Get(ctx, req.NamespacedName, todo)
	if err != nil {
		if errors.IsNotFound(err) {
			Log.Info("Object not found.")
			return ctrl.Result{}, nil
		}
		Log.Error(err, "Error getting todo object")
		return ctrl.Result{Requeue: true}, err
	}
	Log.Info("Got new todo", "todo", todo.ObjectMeta.Name)

	// reconcile config map

	// reconcile deployment config
	dc := &ocpAppsV1.DeploymentConfig{}

	err = r.Client.Get(ctx, types.NamespacedName{Name: todo.GetName(), Namespace: todo.GetNamespace()}, dc)
	if err != nil {
		if errors.IsNotFound(err) {
			fileName := "config/samples/manifests/deploymentconfig.yaml"
			r.applyManifests(ctx, req, todo, dc, fileName)
		} else {
			return ctrl.Result{Requeue: true}, err
		}
		// TODO: should we update then?
	}

	// reconcile service
	svc := &corev1.Service{}
	err = r.Client.Get(ctx, types.NamespacedName{Name: todo.GetName(), Namespace: todo.GetNamespace()}, svc)
	if err != nil {
		if errors.IsNotFound(err) {
			fileName := "config/samples/manifests/service.yaml"
			r.applyManifests(ctx, req, todo, svc, fileName)
		} else {
			return ctrl.Result{Requeue: true}, err
		}
		// TODO: should we update then?
	}

	// reconcile route

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *TodoListReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&applicationv1alpha1.TodoList{}).
		Owns(&ocpAppsV1.DeploymentConfig{}).
		Complete(r)
}

func (r *TodoListReconciler) applyManifests(ctx context.Context, req ctrl.Request, todo *applicationv1alpha1.TodoList, obj client.Object, fileName string) error {

	Log := log.FromContext(ctx)

	b, err := os.ReadFile(fileName)
	if err != nil {
		Log.Error(err, fmt.Sprintf("Couldn't read manifest file for: %s", fileName))
		return err
	}

	if err = yamlutil.Unmarshal(b, &obj); err != nil {
		Log.Error(err, fmt.Sprintf("Couldn't unmarshall yaml file for: %s", fileName))
		return err
	}

	obj.SetNamespace(todo.GetNamespace())
	obj.SetName(todo.GetName())
	controllerutil.SetControllerReference(todo, obj, r.Scheme)

	err = r.Client.Create(ctx, obj)
	if err != nil {
		Log.Error(err, "Couldn't create Deployment Config", "DeploymentConfig", obj.GetName())
		return err
	}

	return nil
}
