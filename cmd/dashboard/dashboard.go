package main

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"fregoli.dev/mister"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type User struct {
	Name string
}

func main() {
	a := mister.GetReduceTaskReply{}
	fmt.Println(a)
	http.HandleFunc("/", handler)
	log.Fatal(http.ListenAndServe(":80", nil))
}

func handler(w http.ResponseWriter, r *http.Request) {
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
	for _, pod := range pods.Items {
		fmt.Fprintf(w, "Pod name:%s", pod.Name)
	}
	// t, err := template.ParseFiles("templates/index.gohtml")
	// if err != nil {
	// 	panic(err)
	// }
	// err = t.Execute(w, struct {
	// 	Name string
	// 	Flag bool
	// }{Name: "David"})
	// if err != nil {
	// 	panic(err)
	// }
}
