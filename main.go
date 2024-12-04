package main

import (
	"context"
	"fmt"
	"html/template"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"log"
	"net/http"
	"os"
)

type PV struct {
	Name         string            `json:"name"`
	Size         string            `json:"size"`
	CreationTime string            `json:"creationTime"`
	UUID         string            `json:"uuid"`
	Labels       map[string]string `json:"labels"`
	Status       string            `json:"status"`
}

type PVC struct {
	Name         string            `json:"name"`
	Namespace    string            `json:"namespace"`
	CreationTime string            `json:"creationTime"`
	UUID         string            `json:"uuid"`
	Labels       map[string]string `json:"labels"`
	Status       string            `json:"status"`
	PVUUID       string            `json:"PVUUID"`
}

type DashboardData struct {
	PVs  []PV  `json:"pvs"`
	PVCs []PVC `json:"pvcs"`
}

func loadK8sData() ([]PV, []PVC, error) {
	kubeconfig := os.Getenv("KUBECONFIG")

	if kubeconfig == "" {
		kubeconfig = "/mnt/jetbrains/work/k8s-pv-dashboard/k8s/.kubeconfig"
	}

	// Create a Kubernetes client
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		log.Fatalf("Error loading kubeconfig: %v", err)
	}

	// Create a Kubernetes clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("Error creating Kubernetes clientset: %v", err)
	}

	// Fetch the PersistentVolumes
	pvClient := clientset.CoreV1().PersistentVolumes()

	// Call the API to get all PVs
	pvs, err := pvClient.List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		log.Fatalf("Error fetching PersistentVolumes: %v", err)
	}
	// Output the PV information
	fmt.Println("PersistentVolumes in the cluster:")
	var pvList []PV
	for _, pv := range pvs.Items {
		quantity := pv.Spec.Capacity["storage"]
		pvList = append(pvList, PV{
			Name:         pv.Name,
			Size:         quantity.String(),
			CreationTime: pv.CreationTimestamp.Time.Format("2006-01-02/15:04:05"),
			UUID:         string(pv.UID),
			Labels:       pv.Labels,
			Status:       string(pv.Status.Phase),
		})
	}

	namespaces, _ := clientset.CoreV1().Namespaces().List(context.TODO(), metav1.ListOptions{})
	var pvcList []PVC

	for _, namespace := range namespaces.Items {
		// Fetch the PersistentVolumes
		pvcClient := clientset.CoreV1().PersistentVolumeClaims(namespace.Name)

		// Call the API to get all PVs
		pvcs, err := pvcClient.List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			log.Fatalf("Error fetching PersistentVolumes: %v", err)
		}
		// Output the PVC information
		for _, pvc := range pvcs.Items {
			pvcList = append(pvcList, PVC{
				Name:         pvc.Name,
				Namespace:    pvc.Namespace,
				CreationTime: pvc.CreationTimestamp.Time.Format("2006-01-02/15:04:05"),
				UUID:         string(pvc.UID),
				Labels:       pvc.Labels,
				Status:       string(pvc.Status.Phase),
				PVUUID:       pvc.Spec.VolumeName,
			})

			podClient := clientset.CoreV1().Pods(pvc.Namespace)
			pods, err := podClient.List(context.TODO(), metav1.ListOptions{
				FieldSelector: "kind=pod",
			})
			if err != nil {
				log.Fatalf("Fehler beim Abrufen von Pods für PVC: %v", err)
			}

			// Über alle Pods gehen und den Node des Pods ermitteln
			for _, pod := range pods.Items {
				fmt.Printf("Pod %s is running on Node %s\n", pod.Name, pod.Spec.NodeName)
			}
		}

	}

	return pvList, pvcList, nil
}

func homeHandler(data DashboardData) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tmpl, err := template.ParseFiles("./templates/index.html")
		if err != nil {
			http.Error(w, "Error loading template", http.StatusInternalServerError)
			log.Printf("Template error: %v\n", err)
			return
		}

		// Execute the template and pass the data
		if err := tmpl.Execute(w, data); err != nil {
			http.Error(w, "Error rendering the template", http.StatusInternalServerError)
			log.Printf("Render error: %v\n", err)
		}
	}
}

func main() {

	pvs, pvcs, err := loadK8sData()
	data := DashboardData{
		PVs:  pvs,
		PVCs: pvcs,
	}
	if err != nil {
		log.Fatalf("Fehler beim Laden der PV-Daten: %v", err)
	}

	// Ausgabe der PV-Daten
	fmt.Printf("Persistent Volumes found in cluster : %d \n", len(pvs))

	// Route for the homepage
	http.HandleFunc("/", homeHandler(data))

	// Start the server
	port := "8080"
	log.Printf("Server running on http://localhost:%s\n", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}
}
