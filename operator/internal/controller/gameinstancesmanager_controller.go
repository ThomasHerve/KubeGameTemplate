/*
Copyright 2026.

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
 
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
 
	appsv1alpha1 "github.com/ThomasHerve/KubeGameTemplate/api/v1alpha1"
)

const frontendComponent = "frontend"

// GameInstancesManagerReconciler reconciles a GameInstancesManager object
type GameInstancesManagerReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=apps.thomas-herve.fr,resources=gameinstancesmanagers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps.thomas-herve.fr,resources=gameinstancesmanagers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps.thomas-herve.fr,resources=gameinstancesmanagers/finalizers,verbs=update
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=events,verbs=create;patch

func (r *GameInstancesManagerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
 
	var gim appsv1alpha1.GameInstancesManager
	if err := r.Get(ctx, req.NamespacedName, &gim); err != nil {
		if errors.IsNotFound(err) {
			// CR supprimée, rien à faire : les objets enfants (Deployment) partent
			// automatiquement via l'ownerReference / garbage collection Kubernetes.
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}
 
	if err := r.reconcileFrontendDeployment(ctx, &gim); err != nil {
		logger.Error(err, "failed to reconcile frontend deployment")
		return ctrl.Result{}, err
	}
 
	return ctrl.Result{}, nil
}

func (r *GameInstancesManagerReconciler) reconcileFrontendDeployment(ctx context.Context, gim *appsv1alpha1.GameInstancesManager) error {
	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deploymentName(gim, frontendComponent),
			Namespace: gim.Namespace,
		},
	}
 
	// CreateOrUpdate : va chercher l'objet, applique la fonction de mutation,
	// puis crée ou patch selon ce qui existe déjà. Idempotent par design.
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, deploy, func() error {
		return r.mutateFrontendDeployment(gim, deploy)
	})
	return err
}

func (r *GameInstancesManagerReconciler) mutateFrontendDeployment(gim *appsv1alpha1.GameInstancesManager, deploy *appsv1.Deployment) error {
	spec := gim.Spec.Frontend
 
	labels := frontendLabels(gim)
 
	deploy.Labels = labels
	deploy.Spec.RevisionHistoryLimit = int32Ptr(1)
	deploy.Spec.Replicas = spec.Replicas
	deploy.Spec.Selector = &metav1.LabelSelector{MatchLabels: labels}
 
	tag := spec.Tag
	if tag == "" {
		tag = "latest"
	}
 
	saName := spec.ServiceAccountName
	if saName == "" {
		saName = serviceAccountName(gim)
	}
 
	deploy.Spec.Template = corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{Labels: labels},
		Spec: corev1.PodSpec{
			ServiceAccountName: saName,
			SecurityContext:    spec.PodSecurityContext,
			Containers: []corev1.Container{
				{
					Name:            frontendComponent,
					Image:           fmt.Sprintf("%s:%s", spec.Repository, tag),
					ImagePullPolicy: spec.PullPolicy,
					SecurityContext: spec.SecurityContext,
					Env: []corev1.EnvVar{
						{Name: "BACKEND_URL", Value: spec.BackendURL},
						{Name: "BACKEND_PROTOCOL", Value: defaultString(spec.BackendProtocol, "https")},
						{Name: "HTTP_ROUTE_ENABLED", Value: fmt.Sprintf("%t", spec.HTTPRouteEnabled)},
					},
					Ports: []corev1.ContainerPort{
						{Name: "http", ContainerPort: spec.Port, Protocol: corev1.ProtocolTCP},
					},
					Resources: spec.Resources,
				},
			},
		},
	}
 
	// Indispensable : sans ownerReference, la suppression de la CR ne
	// nettoie pas le Deployment, et controller-runtime ne peut pas
	// re-déclencher un Reconcile quand quelqu'un modifie le Deployment à la main.
	return controllerutil.SetControllerReference(gim, deploy, r.Scheme)
}
 
func deploymentName(gim *appsv1alpha1.GameInstancesManager, component string) string {
	return fmt.Sprintf("%s-%s", gim.Name, component)
}
 
func serviceAccountName(gim *appsv1alpha1.GameInstancesManager) string {
	return fmt.Sprintf("%s-sa", gim.Name)
}
 
func frontendLabels(gim *appsv1alpha1.GameInstancesManager) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":     frontendComponent,
		"app.kubernetes.io/instance": gim.Name,
		"app.kubernetes.io/part-of":  "GameInstancesManager",
	}
}
 
func int32Ptr(i int32) *int32 { return &i }
 
func defaultString(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

// SetupWithManager sets up the controller with the Manager.
func (r *GameInstancesManagerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appsv1alpha1.GameInstancesManager{}).
		Named("gameinstancesmanager").
		Complete(r)
}
