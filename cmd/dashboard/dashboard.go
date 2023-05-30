package main

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"

	mr "fregoli.dev/mister"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func main() {
	http.HandleFunc("/", handler)
	http.HandleFunc("/api/job", getJob)
	log.Fatal(http.ListenAndServe(":80", nil))
}

func getJob(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	job, err := mr.CallGetJob()
	if err != nil {
		json.NewEncoder(w).Encode(struct{ Error string }{Error: err.Error()})
	}
	json.NewEncoder(w).Encode(job)
}

func handler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	config, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}
	pods, err := clientset.CoreV1().Pods("default").List(context.TODO(), meta.ListOptions{})
	if err != nil {
		panic(err.Error())
	}
	t, err := template.ParseFiles("templates/index.gohtml")
	if err != nil {
		panic(err)
	}
	for _, pod := range pods.Items {
		fmt.Println("annotations: ", pod.GetAnnotations())
	}
	err = t.Execute(w, pods)
	if err != nil {
		panic(err)
	}
}
