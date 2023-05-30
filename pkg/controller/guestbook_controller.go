/*
Copyright 2023.

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
	"os"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	clusterv1 "open-cluster-management.io/api/cluster/v1"

	webappv1 "my.domain/guestbook/api/v1"
)

// GuestbookReconciler reconciles a Guestbook object
type GuestbookReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=webapp.my.domain,resources=guestbooks,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=webapp.my.domain,resources=guestbooks/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=webapp.my.domain,resources=guestbooks/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=namespaces,verbs=create;delete;get;list;update;watch;patch
//+kubebuilder:rbac:groups="apps",resources=deployments,verbs=create;delete;get;list;update;watch;patch
//+kubebuilder:rbac:groups="",resources=services,verbs=create;delete;get;list;update;watch;patch
//+kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=create;delete;get;list;update;watch;patch
//+kubebuilder:rbac:groups="",resources=secrets,verbs=create;delete;get;list;update;watch;patch
//+kubebuilder:rbac:groups=policy.open-cluster-management.io,resources=policies,verbs=create;delete;get;list;update;watch;patch
//+kubebuilder:rbac:groups=policy.open-cluster-management.io,resources=placementbindings,verbs=create;delete;get;list;update;watch;patch
//+kubebuilder:rbac:groups=cluster.open-cluster-management.io,resources=placements,verbs=create;delete;get;list;update;watch;patch
//+kubebuilder:rbac:groups=cluster.open-cluster-management.io,resources=managedclusters,verbs=create;delete;get;list;update;watch;patch
//+kubebuilder:rbac:groups=cluster.open-cluster-management.io,resources=managedclustersets,verbs=create;delete;get;list;update;watch;patch
//+kubebuilder:rbac:groups=cluster.open-cluster-management.io,resources=managedclustersets/join,verbs=create;delete
//+kubebuilder:rbac:groups=cluster.open-cluster-management.io,resources=managedclustersets/bind,verbs=create;delete
//+kubebuilder:rbac:groups=addon.open-cluster-management.io,resources=clustermanagementaddons,verbs=create;delete;get;list;update;watch;patch
//+kubebuilder:rbac:groups=addon.open-cluster-management.io,resources=clustermanagementaddons/finalizers,verbs=update
//+kubebuilder:rbac:groups=addon.open-cluster-management.io,resources=managedclusteraddons,verbs=create;delete;get;list;update;watch;patch
//+kubebuilder:rbac:groups=addon.open-cluster-management.io,resources=managedclusteraddons/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Guestbook object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.4/pkg/reconcile
func (r *GuestbookReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	guestbook := &webappv1.Guestbook{}
	if err := r.Get(ctx, req.NamespacedName, guestbook); err != nil {
		log.Error(err, "unable to fetch Guestbook")
		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// SSA
	mcl := &clusterv1.ManagedCluster{}
	if err := r.Get(ctx, types.NamespacedName{
		Name: "test",
	}, mcl); err != nil {
		log.Error(err, "unable to fetch ManagedCluster")
		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	mcl.Spec.HubAcceptsClient = !mcl.Spec.HubAcceptsClient
	updateOpts := &client.UpdateOptions{FieldManager: "testing"}
	if err := r.Client.Update(ctx, mcl, updateOpts); err != nil {
		log.Error(err, "SSA: failed to update ManagedCluster with SSA")
		return ctrl.Result{}, err
	}
	log.Info("SSA: updated ManagedCluster with SSA")

	mclPatch := []byte(fmt.Sprintf(`{"apiVersion":"cluster.open-cluster-management.io/v1","kind":"ManagedCluster","spec":{"hubAcceptsClient":%t}}`, !mcl.Spec.HubAcceptsClient))
	mclRawPatch := client.RawPatch(types.ApplyPatchType, mclPatch)
	patchOpts := []client.PatchOption{client.ForceOwnership, client.FieldOwner("testing")}
	if err := r.Client.Patch(ctx, mcl, mclRawPatch, patchOpts...); err != nil {
		log.Error(err, "SSA: failed to patch ManagedCluster with SSA")
		return ctrl.Result{}, err
	}
	log.Info("SSA: patched ManagedCluster with SSA")

	srt := &corev1.Secret{}
	ns := os.Getenv("POD_NAMESPACE")
	if ns == "" {
		return ctrl.Result{}, fmt.Errorf("Empty env POD_NAMESPACE")
	}
	if err := r.Get(ctx, types.NamespacedName{
		Namespace: ns,
		Name:      "test",
	}, srt); err != nil {
		log.Error(err, "unable to fetch Secret")
		newSrt := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: ns,
				Name:      "test",
			},
			Data: map[string][]byte{
				"foo": []byte("bar"),
				"key": []byte("value"),
			},
		}
		if err := r.Create(ctx, newSrt); err != nil {
			log.Error(err, "SSA: failed to create Secret")
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	srt.Data["foo"] = []byte("goo")
	if err := r.Client.Update(ctx, srt, updateOpts); err != nil {
		log.Error(err, "SSA: failed to update Secret with SSA")
		return ctrl.Result{}, err
	}
	log.Info("SSA: updated Secret with SSA")

	srtPatch := []byte(`{"apiVersion":"v1","kind":"Secret","data":{"key":"bmV3X3ZhbHVlCg=="}}`)
	srtRawPatch := client.RawPatch(types.ApplyPatchType, srtPatch)
	if err := r.Client.Patch(ctx, srt, srtRawPatch, patchOpts...); err != nil {
		log.Error(err, "SSA: failed to patch Secret with SSA")
		return ctrl.Result{}, err
	}
	log.Info("SSA: patched Secret with SSA")

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *GuestbookReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&webappv1.Guestbook{}).
		Complete(r)
}
