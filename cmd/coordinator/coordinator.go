package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/rpc"

	batch "k8s.io/api/batch/v1"
	api "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

//	func (c *Coordinator) SendJob(args *SendJobArgs, reply *SendJobReply) error {
//		c.QueueMu.Lock()
//		defer c.QueueMu.Unlock()
//		for i := range c.Queue {
//			task := &c.Queue[i]
//			c.PhaseMu.Lock()
//			phase := c.Phase
//			c.PhaseMu.Unlock()
//			if task.State == 0 && task.Type == phase {
//				task.State = 1
//				task.Started = time.Now().Unix()
//				reply.Task = *task
//				fmt.Println("Sending Task " + task.Uid)
//				break
//			}
//		}
//		return nil
//	}

type Coordinator struct{}

// var uid int = 0

type MapTask struct {
	Uid        string
	InputFile  string
	SpreadOver int
}

type GetMapTaskReply struct {
	MapTask
	Done  bool
	Found bool
}

var found = true

func (c *Coordinator) GetMapTask(args *struct{}, reply *GetMapTaskReply) error {
	reply.Found = found
	found = false
	reply.MapTask.Uid = "2"
	reply.MapTask.InputFile = "/files/pg-grimm.txt"
	reply.MapTask.SpreadOver = 2
	fmt.Println("in")
	return nil
}
func toPtr(s api.HostPathType) *api.HostPathType {
	return &s
}
func main() {
	// creates the in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}
	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}
	batchClient := clientset.BatchV1().Jobs("default")
	job := &batch.Job{
		ObjectMeta: meta.ObjectMeta{
			Name: "wordcount",
		},
		Spec: batch.JobSpec{
			Parallelism: int32Ptr(2),
			Template: api.PodTemplateSpec{
				Spec: api.PodSpec{
					Volumes: []api.Volume{{
						Name: "files",
						VolumeSource: api.VolumeSource{
							PersistentVolumeClaim: &api.PersistentVolumeClaimVolumeSource{
								ClaimName: "misterpvc",
							},
						},
					}},
					Containers: []api.Container{
						{
							Env: []api.EnvVar{{
								Name: "MISTER_POD_NAME",
								ValueFrom: &api.EnvVarSource{
									FieldRef: &api.ObjectFieldSelector{FieldPath: "metadata.name"},
								},
							}, {
								Name:  "MISTER_WORKER_PHASE",
								Value: "map",
							}, {
								Name:  "MISTER_PARTITIONS",
								Value: "2",
							}},
							VolumeMounts: []api.VolumeMount{{
								Name:      "files",
								MountPath: "/files",
							}},
							Name:  "wordcount",
							Image: "wordcount:1",
						},
					},
					RestartPolicy: api.RestartPolicyNever,
				},
			},
		},
	}
	fmt.Println("Creating job...")
	result, err := batchClient.Create(context.TODO(), job, meta.CreateOptions{})
	if err != nil {
		panic(err)
	}
	fmt.Printf("Created job %q.\n", result.GetObjectMeta().GetName())

	// get pods in all the namespaces by omitting namespace
	// Or specify namespace to get pods in particular namespace
	pods, err := clientset.CoreV1().Pods("default").List(context.TODO(), meta.ListOptions{})
	if err != nil {
		panic(err.Error())
	}
	fmt.Printf("There are %d pods in the cluster\n", len(pods.Items))

	c := Coordinator{}
	// start a thread that listens for RPCs from worker.go
	rpc.Register(&c)
	rpc.HandleHTTP()
	l, e := net.Listen("tcp", ":1234")
	if e != nil {
		log.Fatal("listen error:", e)
	}
	http.Serve(l, nil)
}

func int32Ptr(i int32) *int32 { return &i }
