package api

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	appsv1alpha1 "github.com/ThomasHerve/KubeGameTemplate/api/v1alpha1"
)

var kubeClient client.Client
var scheme *runtime.Scheme

func SetClient(c client.Client, s *runtime.Scheme) {
	kubeClient = c
	scheme = s
}

// ============================================================================
// HTTP SERVER
// ============================================================================

func StartHTTPServer() {

	http.HandleFunc("/create-room", handleCreateRoom)

	http.HandleFunc("/delete-room", handleDeleteRoom)

	fmt.Println("HTTP API listening on :80")

	if err := http.ListenAndServe(":80", nil); err != nil {
		panic(err)
	}
}

func handleCreateRoom(w http.ResponseWriter, r *http.Request) {
	if kubeClient == nil {
		http.Error(w, "kubernetes client not initialized", http.StatusInternalServerError)
		return
	}

	url := r.Host 

	ctx := context.Background()

	// On ne sais pas encore sur quel GameInstancesManager on veut créer le pod, donc on va tous les lister et deduire à partir de l'url lequel est le bon
	var gimList appsv1alpha1.GameInstancesManagerList
	var gim appsv1alpha1.GameInstancesManager

	if err := kubeClient.List(ctx, &gimList); err != nil {
		http.Error(w, fmt.Sprintf("failed to list GameInstancesManagers: %v", err), http.StatusInternalServerError)
		return
	}

	for _, gim_loop := range gimList.Items {
		if gim_loop.Spec.Hostname == url {
			gim = gim_loop
			break
		}
	}

	if gim.Name == "" {
		http.Error(w, fmt.Sprintf("no GameInstancesManager found for hostname: %s", url), http.StatusNotFound)
		return
	}

	// Check max instances limit
	if gim.Spec.MaxInstances != nil {
		currentInstances, err := countExistingInstances(ctx, gim)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to count existing instances: %v", err), http.StatusInternalServerError)
			return
		}
		if currentInstances >= int(*gim.Spec.MaxInstances) {
			http.Error(w, fmt.Sprintf("maximum number of instances (%d) reached", *gim.Spec.MaxInstances), http.StatusTooManyRequests)
			return
		}
	}

	repository := gim.Spec.Instance.Repository
	pullPolicy := gim.Spec.Instance.PullPolicy
	tag := gim.Spec.Instance.Tag
	internalPort := gim.Spec.Instance.InternalPort

	instanceID := randomID(5)
	deploymentName := fmt.Sprintf("%s-instance-%s", gim.Name, instanceID)
	password := randomID(16)
	hostKey := randomID(16)
	instanceLabelValue := "game-instance"
	labels := map[string]string{
		"app.kubernetes.io/name":                 instanceLabelValue,
		"app.kubernetes.io/part-of":              "GameInstancesManager",
		"gameinstancesmanager":                   gim.Name,
		"gameinstancesmanager.io/instance-id":    instanceID,
	}

	backendURL := fmt.Sprintf("kube-game-operator-service.%s.svc.cluster.local", gim.Namespace) //strings.TrimSpace(gim.Spec.Hostname)
	/*if backendURL == "" {
		backendURL = fmt.Sprintf("%s.%s.svc.cluster.local", gim.Name, gim.Namespace)
	}*/

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deploymentName,
			Namespace: gim.Namespace,
			Labels:    labels,
			Annotations: map[string]string{
				"gameinstancesmanager.io/instance-id": instanceID,
				"gameinstancesmanager.io/password":    password,
				"gameinstancesmanager.io/host-key":    hostKey,
			},
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(&gim, appsv1alpha1.GroupVersion.WithKind("GameInstancesManager")),
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: int32Ptr(1),
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyAlways,
					Containers: []corev1.Container{
						{
							Name:  "example",
							Image: fmt.Sprintf("%s:%s", repository, tag),
							Ports: []corev1.ContainerPort{{ContainerPort: internalPort}},
							ImagePullPolicy: pullPolicy,
							Env: []corev1.EnvVar{
								{Name: "INSTANCE_NAME", Value: instanceID},
								{Name: "BACKEND_URL", Value: backendURL},
								{Name: "PASSWORD", Value: password},
								{Name: "HOST_KEY", Value: hostKey},
							},
						},
					},
				},
			},
		},
	}

	if err := kubeClient.Create(ctx, deployment); err != nil {
		http.Error(w, fmt.Sprintf("failed to create deployment: %v", err), http.StatusInternalServerError)
		return
	}

	controllerutil.SetControllerReference(&gim, deployment, scheme)

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-service", deploymentName),
			Namespace: gim.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/name":    instanceLabelValue,
				"app.kubernetes.io/part-of": "GameInstancesManager",
				"gameinstancesmanager":      gim.Name,
			},
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(&gim, appsv1alpha1.GroupVersion.WithKind("GameInstancesManager")),
			},
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"app.kubernetes.io/name":                 instanceLabelValue,
				"app.kubernetes.io/part-of":              "GameInstancesManager",
				"gameinstancesmanager":                   gim.Name,
				"gameinstancesmanager.io/instance-id":    instanceID,
			},
			Ports: []corev1.ServicePort{
				{
					Port:       80,
					TargetPort: intstr.FromInt(int(internalPort)),
				},
			},
		},
	}

	if err := kubeClient.Create(ctx, service); err != nil {
		http.Error(w, fmt.Sprintf("failed to create service: %v", err), http.StatusInternalServerError)
		return
	}

	routePath := "/" + instanceID
	pathType := gatewayv1.PathMatchPathPrefix
	pathValue := routePath
	httpRoute := &gatewayv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instanceID,
			Namespace: gim.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/name":    "game-instance-route",
				"app.kubernetes.io/part-of": "GameInstancesManager",
				"gameinstancesmanager":      gim.Name,
			},
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(&gim, appsv1alpha1.GroupVersion.WithKind("GameInstancesManager")),
			},
		},
		Spec: gatewayv1.HTTPRouteSpec{
			CommonRouteSpec: gatewayv1.CommonRouteSpec{
				ParentRefs: []gatewayv1.ParentReference{{
					Name: gatewayv1.ObjectName("operator-gateway"),
				}},
			},
			Rules: []gatewayv1.HTTPRouteRule{{
				Matches: []gatewayv1.HTTPRouteMatch{{
					Path: &gatewayv1.HTTPPathMatch{
						Type:  &pathType,
						Value: &pathValue,
					},
				}},
				BackendRefs: []gatewayv1.HTTPBackendRef{{
					BackendRef: gatewayv1.BackendRef{
						BackendObjectReference: gatewayv1.BackendObjectReference{
							Name:      gatewayv1.ObjectName(service.Name),
							Namespace: func() *gatewayv1.Namespace { ns := gatewayv1.Namespace(gim.Namespace); return &ns }(),
							Port:     func() *gatewayv1.PortNumber { p := gatewayv1.PortNumber(80); return &p }(),
						},
						Weight: func() *int32 { w := int32(1); return &w }(),
					},
				}},
			}},
		},
	}

	if err := kubeClient.Create(ctx, httpRoute); err != nil {
		http.Error(w, fmt.Sprintf("failed to create httproute: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	response := map[string]string{
		"instance": instanceID,
		"host_key": hostKey,
	}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, fmt.Sprintf("failed to encode response: %v", err), http.StatusInternalServerError)
	}

	controllerutil.SetControllerReference(&gim, service, scheme)
}

func handleDeleteRoom(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, fmt.Sprintf("failed to parse form: %v", err), http.StatusBadRequest)
		return
	}

	instanceName := strings.TrimSpace(r.FormValue("instance"))
	password := strings.TrimSpace(r.FormValue("password"))
	if instanceName == "" || password == "" {
		var payload map[string]string
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "missing instance or password", http.StatusBadRequest)
			return
		}
		instanceName = strings.TrimSpace(payload["instance"])
		password = strings.TrimSpace(payload["password"])
	}

	if instanceName == "" || password == "" {
		http.Error(w, "missing instance or password", http.StatusBadRequest)
		return
	}

	ctx := context.Background()
	deployment := &appsv1.Deployment{}
	namespace := strings.TrimSpace(r.FormValue("namespace"))
	if namespace == "" {
		namespace = strings.TrimSpace(r.URL.Query().Get("namespace"))
	}

	var deploymentList appsv1.DeploymentList
	if err := kubeClient.List(ctx, &deploymentList, client.InNamespace(namespace)); err != nil {
		if namespace == "" {
			if err := kubeClient.List(ctx, &deploymentList); err != nil {
				http.Error(w, fmt.Sprintf("failed to list deployments: %v", err), http.StatusInternalServerError)
				return
			}
		} else {
			http.Error(w, fmt.Sprintf("failed to list deployments: %v", err), http.StatusInternalServerError)
			return
		}
	}

	found := false
	for i := range deploymentList.Items {
		candidate := &deploymentList.Items[i]
		if candidate.Annotations["gameinstancesmanager.io/instance-id"] == instanceName || strings.HasSuffix(candidate.Name, fmt.Sprintf("-instance-%s", instanceName)) {
			deployment = candidate
			found = true
			break
		}
	}
	if !found {
		http.Error(w, fmt.Sprintf("deployment for room %s not found", instanceName), http.StatusNotFound)
		return
	}

	storedPassword, ok := deployment.Annotations["gameinstancesmanager.io/password"]
	if !ok || storedPassword != password {
		http.Error(w, "wrong password", http.StatusUnauthorized)
		return
	}

	service := &corev1.Service{}
	serviceName := fmt.Sprintf("%s-service", deployment.Name)
	if err := kubeClient.Get(ctx, client.ObjectKey{Namespace: deployment.Namespace, Name: serviceName}, service); err == nil {
		if err := kubeClient.Delete(ctx, service); err != nil {
			http.Error(w, fmt.Sprintf("failed to delete service: %v", err), http.StatusInternalServerError)
			return
		}
	}

	httpRoute := &gatewayv1.HTTPRoute{}
	if err := kubeClient.Get(ctx, client.ObjectKey{Namespace: deployment.Namespace, Name: instanceName}, httpRoute); err == nil {
		if err := kubeClient.Delete(ctx, httpRoute); err != nil {
			http.Error(w, fmt.Sprintf("failed to delete httproute: %v", err), http.StatusInternalServerError)
			return
		}
	}

	if err := kubeClient.Delete(ctx, deployment); err != nil {
		http.Error(w, fmt.Sprintf("failed to delete deployment: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("room deleted"))
}

func int32Ptr(v int32) *int32 {
	return &v
}

// Fonctions utilitaires
func randomID(length int) (string) {

	const letters = "abcdefghijklmnopqrstuvwxyz"

	b := make([]byte, length)

	if _, err := rand.Read(b); err != nil {
		return ""
	}

	for i := range b {
		b[i] = letters[int(b[i])%len(letters)]
	}

	return string(b)
}

// countExistingInstances counts the number of running game instances for a given GameInstancesManager
func countExistingInstances(ctx context.Context, gim appsv1alpha1.GameInstancesManager) (int, error) {
	var deploymentList appsv1.DeploymentList
	
	// List all deployments in the namespace
	if err := kubeClient.List(ctx, &deploymentList, client.InNamespace(gim.Namespace)); err != nil {
		return 0, err
	}

	count := 0
	// Count deployments belonging to this GameInstancesManager
	for _, deployment := range deploymentList.Items {
		if label, ok := deployment.Labels["gameinstancesmanager"]; ok && label == gim.Name {
			// Only count running instances (check if deployment has replicas)
			if deployment.Status.Replicas > 0 {
				count++
			}
		}
	}

	return count, nil
}