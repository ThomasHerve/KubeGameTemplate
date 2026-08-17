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
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

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

	if err := r.reconcileFrontendGateway(ctx, gim); err != nil {
		return ctrl.Result{}, err
	}

	if gim.Spec.Frontend.Enabled {
		if err := r.reconcileFrontendServiceAccount(ctx, &gim); err != nil {
			logger.Error(err, "failed to reconcile frontend serviceaccount")
			return ctrl.Result{}, err
		}
	
		if err := r.reconcileFrontendService(ctx, &gim); err != nil {
			logger.Error(err, "failed to reconcile frontend service")
			return ctrl.Result{}, err
		}
	
		if err := r.reconcileFrontendDeployment(ctx, &gim); err != nil {
			logger.Error(err, "failed to reconcile frontend deployment")
			return ctrl.Result{}, err
		}

		if err := r.reconcileFrontendHTTPRoute(ctx, &gim); err != nil {
			logger.Error(err, "failed to reconcile http route deployment")
			return ctrl.Result{}, err
		}
	}
 
	return ctrl.Result{}, nil
}

func (r *GameInstancesManagerReconciler) reconcileFrontendServiceAccount(ctx context.Context, gim *appsv1alpha1.GameInstancesManager) error {
	// Si l'utilisateur a fourni un SA existant via le Spec, on ne le gère pas
	// (il est censé exister déjà, potentiellement en dehors du scope de l'operator).
	if gim.Spec.Frontend.ServiceAccountName != "" {
		return nil
	}
 
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceAccountName(gim),
			Namespace: gim.Namespace,
		},
	}
 
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, sa, func() error {
		sa.Labels = frontendLabels(gim)
		return controllerutil.SetControllerReference(gim, sa, r.Scheme)
	})
	return err
}
 
func (r *GameInstancesManagerReconciler) reconcileFrontendService(ctx context.Context, gim *appsv1alpha1.GameInstancesManager) error {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deploymentName(gim, frontendComponent),
			Namespace: gim.Namespace,
		},
	}
 
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, svc, func() error {
		labels := frontendLabels(gim)
		svc.Labels = labels
		svc.Spec.Selector = labels
		svc.Spec.Type = corev1.ServiceTypeClusterIP
		svc.Spec.Ports = []corev1.ServicePort{
			{
				Name:       "http",
				Port:       gim.Spec.Frontend.ExternalPort,
				TargetPort: intstrFromPort(gim.Spec.Frontend.Port),
				Protocol:   corev1.ProtocolTCP,
			},
		}
		return controllerutil.SetControllerReference(gim, svc, r.Scheme)
	})
	return err
}

func (r *GameInstancesManagerReconciler) reconcileFrontendGateway(
	ctx context.Context,
	gim *appsv1alpha1.GameInstancesManager,
) error {
	gateway := &gatewayv1.Gateway{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "operator-gateway",
			Namespace: gim.Namespace,
		},
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, gateway, func() error {
		gateway.Labels = frontendLabels(gim)

		gateway.Spec = gatewayv1.GatewaySpec{
			GatewayClassName: gatewayv1.ObjectName("traefik"),

			Listeners: []gatewayv1.Listener{
				{
					Name:     gatewayv1.SectionName("websecure"),
					Protocol: gatewayv1.HTTPSProtocolType,
					Port:     gatewayv1.PortNumber(8443),

					Hostname: func() *gatewayv1.Hostname {
						h := gatewayv1.Hostname(gim.Spec.Frontend.Hostname)
						return &h
					}(),

					AllowedRoutes: gatewayv1.AllowedRoutes{
						Namespaces: &gatewayv1.RouteNamespaces{
							From: func() *gatewayv1.NamespaceFrom {
								from := gatewayv1.NamespacesFromAll
								return &from
							}(),
						},
					},

					TLS: &gatewayv1.ListenerTLSConfig{
						Mode: func() *gatewayv1.TLSModeType {
							mode := gatewayv1.TLSModeTerminate
							return &mode
						}(),

						CertificateRefs: []gatewayv1.LocalObjectReference{
							{
								Name: gatewayv1.ObjectName("frontend-server-tls"),
							},
						},
					},
				},
			},
		}

		return controllerutil.SetControllerReference(gim, gateway, r.Scheme)
	})

	return err
}

func (r *GameInstancesManagerReconciler) reconcileFrontendHTTPRoute(
	ctx context.Context,
	gim *appsv1alpha1.GameInstancesManager,
) error {
	httpRoute := &gatewayv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deploymentName(gim, frontendComponent),
			Namespace: gim.Namespace,
		},
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, httpRoute, func() error {
		httpRoute.Labels = frontendLabels(gim)

		httpRoute.Spec = gatewayv1.HTTPRouteSpec{
			CommonRouteSpec: gatewayv1.CommonRouteSpec{
				ParentRefs: []gatewayv1.ParentReference{
					{
						Name: gatewayv1.ObjectName("operator-gateway"),
					},
				},
			},

			Hostnames: []gatewayv1.Hostname{
				gatewayv1.Hostname(gim.Spec.Frontend.Hostname),
			},

			Rules: []gatewayv1.HTTPRouteRule{
				{
					Matches: []gatewayv1.HTTPRouteMatch{
						{
							Path: &gatewayv1.HTTPPathMatch{
								Type: func() *gatewayv1.PathMatchType {
									t := gatewayv1.PathMatchPathPrefix
									return &t
								}(),
								Value: func() *string {
									v := "/"
									return &v
								}(),
							},
						},
					},

					BackendRefs: []gatewayv1.HTTPBackendRef{
						{
							BackendRef: gatewayv1.BackendRef{
								BackendObjectReference: gatewayv1.BackendObjectReference{
									Name: gatewayv1.ObjectName(
										deploymentName(gim, frontendComponent),
									),
									Port: func() *gatewayv1.PortNumber {
										p := gatewayv1.PortNumber(
											gim.Spec.Frontend.ExternalPort,
										)
										return &p
									}(),
								},

								Weight: func() *int32 {
									w := int32(1)
									return &w
								}(),
							},
						},
					},
				},
			},
		}

		return controllerutil.SetControllerReference(gim, httpRoute, r.Scheme)
	})

	return err
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
 
func intstrFromPort(port int32) intstr.IntOrString {
	return intstr.FromInt32(port)
}

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
		Owns(&appsv1.Deployment{}).      // ré-déclenche Reconcile si le Deployment est modifié/supprimé manuellement
		Owns(&corev1.ServiceAccount{}).
		Owns(&corev1.Service{}).
		Complete(r)
}
