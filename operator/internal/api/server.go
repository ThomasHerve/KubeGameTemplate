package api

import (
	"context"
	"crypto/rand"
	"fmt"
	"net/http"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/ThomasHerve/KubeGameTemplate/api/v1alpha1"
)

var kubeClient client.Client

func SetClient(c client.Client) {
	kubeClient = c
}

// ============================================================================
// HTTP SERVER
// ============================================================================

func StartHTTPServer() {

	http.HandleFunc("/create-room", handleCreateRoom)

	http.HandleFunc("/delete-room", handleDeleteRoom)

	fmt.Println("HTTP API listening on :8080")

	if err := http.ListenAndServe(":8080", nil); err != nil {
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
	pod_id := randomID(5)
	podName := fmt.Sprintf("%s-instance-%s", gim.Name, pod_id)
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: gim.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/name":       "example-pod",
				"app.kubernetes.io/part-of":    "GameInstancesManager",
				"gameinstancesmanager":         gim.Name,
			},
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(&gim, appsv1alpha1.GroupVersion.WithKind("GameInstancesManager")),
			},
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
			Containers: []corev1.Container{
				{
					Name:  "example",
					Image: "nginx:latest",
					Ports: []corev1.ContainerPort{{ContainerPort: 80}},
				},
			},
		},
	}

	if err := kubeClient.Create(ctx, pod); err != nil {
		http.Error(w, fmt.Sprintf("failed to create pod: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	_, _ = w.Write([]byte(fmt.Sprintf("pod %s created for GameInstancesManager %s/%s", pod.Name, gim.Namespace, gim.Name)))
}


func handleDeleteRoom(w http.ResponseWriter, r *http.Request) {

	
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