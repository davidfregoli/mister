package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/rpc"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	mr "fregoli.dev/mister"

	batch "k8s.io/api/batch/v1"
	api "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type Coordinator struct {
	Phase        string
	Mappers      int
	Reducers     int
	MapQueue     []*mr.MapTask
	MapQueueMx   sync.RWMutex
	MapRunners   []*mr.MapTask
	MapRunnersMx sync.RWMutex
	TaskCounter  int64
}

type MapQueue []mr.MapTask

func (c *Coordinator) ShiftMapQueue() *mr.MapTask {
	c.MapQueueMx.Lock()
	defer c.MapQueueMx.Unlock()
	if len(c.MapQueue) == 0 {
		return nil
	}
	task := c.MapQueue[0]
	c.MapQueue = c.MapQueue[1:]
	return task
}

func (c *Coordinator) RunMap(task *mr.MapTask) {
	c.MapRunnersMx.Lock()
	defer c.MapRunnersMx.Unlock()
	c.MapRunners = append(c.MapRunners, task)
}

func (c *Coordinator) GetMapTask(args *mr.GetMapTaskArgs, reply *mr.GetMapTaskReply) error {
	task := c.ShiftMapQueue()
	if task == nil {
		reply.Done = c.CheckMapDone()
		return nil
	}
	task.Reducers = c.Reducers
	task.Worker = args.Worker
	task.Started = time.Now().Unix()
	c.RunMap(task)
	reply.Found = true
	reply.MapTask = *task
	fmt.Println(reply.MapTask.InputFile)
	return nil
}

func (c *Coordinator) NotifyCompleted(args *mr.NotifyCompoletedArgs, reply *mr.Stub) error {
	c.MapRunnersMx.Lock()
	defer c.MapRunnersMx.Unlock()
	nu := make([]*mr.MapTask, len(c.MapRunners)-1)
	j := 0
	for i, task := range c.MapRunners {
		if task.Uid == args.Uid {
			continue
		}
		nu[j] = c.MapRunners[i]
		j++
	}
	c.MapRunners = nu
	return nil
}

func (c *Coordinator) CheckMapDone() bool {
	c.MapQueueMx.RLock()
	c.MapRunnersMx.RLock()
	defer c.MapQueueMx.RUnlock()
	defer c.MapRunnersMx.RUnlock()
	emptyQueue := len(c.MapQueue) == 0
	emptyRunners := len(c.MapQueue) == 0
	return emptyQueue && emptyRunners
}

func main() {
	cord := new(Coordinator)

	err := createJob(cord)
	if err != nil {
		log.Fatal(err)
	}
	spawnMappers(cord)

	rpc.Register(cord)
	rpc.HandleHTTP()
	l, e := net.Listen("tcp", ":1234")
	if e != nil {
		log.Fatal("listen error:", e)
	}
	http.Serve(l, nil)
}

func createJob(c *Coordinator) error {
	job := struct {
		Mappers  int
		Reducers int
		Path     string
	}{
		Mappers:  16,
		Reducers: 16,
		Path:     "/files/input/",
	}
	c.Mappers = job.Mappers
	c.Reducers = job.Reducers
	files, err := os.ReadDir(job.Path)
	if err != nil {
		return err
	}
	c.MapQueue = make([]*mr.MapTask, len(files))
	for i, file := range files {
		uid := atomic.AddInt64(&c.TaskCounter, 1)
		path := job.Path + file.Name()
		if err != nil {
			return err
		}
		c.MapQueue[i] = &mr.MapTask{
			Uid:       strconv.FormatInt(uid, 10),
			InputFile: path,
		}
	}
	return nil
}

func spawnMappers(c *Coordinator) {
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
			Parallelism: int32Ptr(int32(c.Mappers)),
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
								Name:  "MISTER_REDUCERS",
								Value: strconv.Itoa(c.Reducers),
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

}

func int32Ptr(i int32) *int32 { return &i }
