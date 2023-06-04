package main

import (
	"bytes"
	"context"
	"encoding/json"
	"html/template"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"

	mr "fregoli.dev/mister"
	api "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func main() {
	fs := http.FileServer(http.Dir("/files"))
	http.Handle("/files/", http.StripPrefix("/files", fs))
	http.HandleFunc("/api/job", getJob)
	http.HandleFunc("/api/logs/pod", getPodLogs)
	http.HandleFunc("/api/logs/coordinator", getCoordinatorLogs)
	http.HandleFunc("/", handler)
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

func getPodLogs(w http.ResponseWriter, r *http.Request) {
	params := r.URL.Query()
	podParam, ok := params["podname"]
	if !ok {
		log.Fatal("no pod name provided")
	}
	podname := podParam[0]
	getPodLogsByName(podname, w)
}

func getPodLogsByName(podname string, w http.ResponseWriter) {
	config, err := rest.InClusterConfig()
	if err != nil {
		json.NewEncoder(w).Encode(struct{ Error string }{Error: err.Error()})
	}
	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		json.NewEncoder(w).Encode(struct{ Error string }{Error: err.Error()})
	}
	req := client.CoreV1().Pods("default").GetLogs(podname, &api.PodLogOptions{})
	podLogs, err := req.Stream(context.TODO())
	if err != nil {
		json.NewEncoder(w).Encode(struct{ Error string }{Error: "error in opening stream"})
	}
	defer podLogs.Close()

	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, podLogs)
	if err != nil {
		json.NewEncoder(w).Encode(struct{ Error string }{Error: "error in copy information from podLogs to buf"})
	}
	str := buf.String()

	json.NewEncoder(w).Encode(strings.Split(str, "\n"))

}

func getCoordinatorLogs(w http.ResponseWriter, r *http.Request) {
	config, err := rest.InClusterConfig()
	if err != nil {
		json.NewEncoder(w).Encode(struct{ Error string }{Error: err.Error()})
	}
	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		json.NewEncoder(w).Encode(struct{ Error string }{Error: err.Error()})
	}
	pods, err := client.CoreV1().Pods("default").List(context.TODO(), meta.ListOptions{})
	if err != nil {
		json.NewEncoder(w).Encode(struct{ Error string }{Error: err.Error()})
	}
	test := regexp.MustCompile("^coordinator-")
	for _, pod := range pods.Items {
		if !test.MatchString(pod.Name) {
			continue
		}
		getPodLogsByName(pod.Name, w)
		return
	}
	json.NewEncoder(w).Encode(struct{ Error string }{Error: "unable to find coordinator pod"})
}

func handler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	config, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}
	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}
	pods, err := client.CoreV1().Pods("default").List(context.TODO(), meta.ListOptions{})
	if err != nil {
		panic(err.Error())
	}
	t, err := template.ParseFiles("templates/index.gohtml")
	if err != nil {
		panic(err)
	}
	// for _, pod := range pods.Items {
	// 	fmt.Println("annotations: ", pod.GetAnnotations())
	// }
	err = t.Execute(w, pods)
	if err != nil {
		panic(err)
	}
}
