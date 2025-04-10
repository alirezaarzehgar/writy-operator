/*
Copyright 2025.

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
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	apiv1 "github.com/alirezaarzehgar/writy-operator/api/v1"
	operatorv1 "github.com/alirezaarzehgar/writy-operator/api/v1"
)

var (
	requeueDuration = time.Second * 10
)

// WrityClusterReconciler reconciles a WrityCluster object
type WrityClusterReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=operator.writy.io,resources=writyclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=operator.writy.io,resources=writyclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=operator.writy.io,resources=writyclusters/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the WrityCluster object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.19.0/pkg/reconcile
func (r *WrityClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	writyCluster := &apiv1.WrityCluster{}
	err := r.Get(ctx, req.NamespacedName, writyCluster)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("WrityCluster resource not found", "error", err)
			return ctrl.Result{}, nil
		}

		logger.Info("failed to get WrityCluster resource", "error", err)
		return ctrl.Result{RequeueAfter: requeueDuration}, err
	}

	if writyCluster.Spec.Size == nil {
		logger.Info("no size defined: set Spec.Size to 1")
		*writyCluster.Spec.Size = 1
	}

	if *writyCluster.Spec.Size == 0 {
		logger.Info("Skip the next steps: WrityCluster Size is 0")
		return ctrl.Result{}, nil
	}

	var stfs appsv1.StatefulSet
	err = r.Get(ctx, req.NamespacedName, &stfs)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("create StatefulSet if not exists", "WrityCluster", writyCluster)

			err := reconcielWrityCluster(ctx, logger, writyCluster, r.Client)
			return ctrl.Result{}, err
		} else {
			logger.Info("failed to get StatefulSet. Requeue request", "requeue time", requeueDuration, "error", err)
			return ctrl.Result{RequeueAfter: requeueDuration}, err
		}
	}

	logger.Info("update WrityCluster")
	err = reconcielWrityCluster(ctx, logger, writyCluster, r.Client)
	if err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *WrityClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&operatorv1.WrityCluster{}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Complete(r)
}
