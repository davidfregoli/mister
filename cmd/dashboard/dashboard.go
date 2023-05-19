package main

import (
	"context"
	"fmt"

	apps "k8s.io/api/apps/v1"
	api "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func main() {
	config, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}
	deploymentsClient := clientset.AppsV1().Deployments("default")
	deployment := &apps.Deployment{
		ObjectMeta: meta.ObjectMeta{
			Name: "wordcount",
		},
		Spec: apps.DeploymentSpec{
			Replicas: int32Ptr(2),
			Selector: &meta.LabelSelector{
				MatchLabels: map[string]string{
					"app": "wordcount",
				},
			},
			Template: api.PodTemplateSpec{
				ObjectMeta: meta.ObjectMeta{
					Labels: map[string]string{
						"app": "wordcount",
					},
				},
				Spec: api.PodSpec{
					Containers: []api.Container{
						{
							Name:  "wordcount",
							Image: "wordcount:1",
							Ports: []api.ContainerPort{},
						},
					},
				},
			},
		},
	}
	fmt.Println("Creating deployment...")
	result, err := deploymentsClient.Create(context.TODO(), deployment, meta.CreateOptions{})
	if err != nil {
		panic(err)
	}
	fmt.Printf("Created deployment %q.\n", result.GetObjectMeta().GetName())
}

func int32Ptr(i int32) *int32 { return &i }
