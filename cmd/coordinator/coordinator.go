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
	Path            string
	Phase           string
	Mappers         int
	Reducers        int
	MapQueue        []*mr.MapTask
	MapQueueMx      sync.RWMutex
	MapRunners      []*mr.MapTask
	MapRunnersMx    sync.RWMutex
	ReduceQueue     []*mr.ReduceTask
	ReduceQueueMx   sync.RWMutex
	ReduceRunners   []*mr.ReduceTask
	ReduceRunnersMx sync.RWMutex
	TaskCounter     int64
	CompletedMaps   []string
	CompletedMapsMx sync.RWMutex
}

type MapQueue []mr.MapTask

func (c *Coordinator) GetMapTask(args *mr.GetMapTaskArgs, reply *mr.GetMapTaskReply) error {
	c.MapQueueMx.Lock()
	defer c.MapQueueMx.Unlock()
	c.MapRunnersMx.Lock()
	defer c.MapRunnersMx.Unlock()
	if len(c.MapQueue) == 0 {
		reply.Done = len(c.MapRunners) == 0
		return nil
	}
	task := c.MapQueue[0]
	c.MapQueue = c.MapQueue[1:]
	task.Reducers = c.Reducers
	task.Worker = args.Worker
	task.Started = time.Now().Unix()
	c.MapRunners = append(c.MapRunners, task)
	reply.Found = true
	reply.MapTask = *task
	fmt.Println(task.Uid, " to ", task.Worker)
	return nil
}

func (c *Coordinator) GetReduceTask(args *mr.GetReduceTaskArgs, reply *mr.GetReduceTaskReply) error {
	c.ReduceQueueMx.Lock()
	defer c.ReduceQueueMx.Unlock()
	c.ReduceRunnersMx.Lock()
	defer c.ReduceRunnersMx.Unlock()
	if len(c.ReduceQueue) == 0 {
		reply.Done = len(c.ReduceRunners) == 0
		return nil
	}
	task := c.ReduceQueue[0]
	c.ReduceQueue = c.ReduceQueue[1:]
	task.Worker = args.Worker
	task.Started = time.Now().Unix()
	c.ReduceRunners = append(c.ReduceRunners, task)
	reply.Found = true
	reply.ReduceTask = *task
	return nil
}

func (c *Coordinator) NotifyCompletedMap(args *mr.NotifyCompoletedArgs, reply *mr.Stub) error {
	c.MapRunnersMx.Lock()
	defer c.MapRunnersMx.Unlock()
	nu := make([]*mr.MapTask, len(c.MapRunners)-1)
	j := 0
	fmt.Println("from: ", args.Worker)
	for i, task := range c.MapRunners {
		if task.Uid == args.Uid {
			c.CompletedMapsMx.Lock()
			c.CompletedMaps = append(c.CompletedMaps, task.Uid)
			c.CompletedMapsMx.Unlock()
			continue
		}
		nu[j] = c.MapRunners[i]
		j++
	}
	c.MapRunners = nu
	return nil
}

func (c *Coordinator) NotifyCompletedReduce(args *mr.NotifyCompoletedArgs, reply *mr.Stub) error {
	c.ReduceRunnersMx.Lock()
	defer c.ReduceRunnersMx.Unlock()
	nu := make([]*mr.ReduceTask, len(c.ReduceRunners)-1)
	j := 0
	for i, task := range c.ReduceRunners {
		if task.Uid == args.Uid {
			continue
		}
		nu[j] = c.ReduceRunners[i]
		j++
	}
	c.ReduceRunners = nu
	return nil
}

func (c *Coordinator) CheckMapDone() bool {
	c.MapQueueMx.RLock()
	defer c.MapQueueMx.RUnlock()
	c.MapRunnersMx.RLock()
	defer c.MapRunnersMx.RUnlock()
	emptyQueue := len(c.MapQueue) == 0
	emptyRunners := len(c.MapRunners) == 0
	return emptyQueue && emptyRunners
}

func (c *Coordinator) CheckReduceDone() bool {
	c.ReduceQueueMx.RLock()
	defer c.ReduceQueueMx.RUnlock()
	c.ReduceRunnersMx.RLock()
	defer c.ReduceRunnersMx.RUnlock()
	emptyQueue := len(c.ReduceQueue) == 0
	emptyRunners := len(c.ReduceRunners) == 0
	return emptyQueue && emptyRunners
}

func main() {
	cord := new(Coordinator)

	cord.Phase = "map"
	err := cord.CreateJob()
	if err != nil {
		log.Fatal(err)
	}
	cord.SpawnMappers()

	go cord.Loop()

	rpc.Register(cord)
	rpc.HandleHTTP()
	l, e := net.Listen("tcp", ":1234")
	if e != nil {
		log.Fatal("listen error:", e)
	}
	http.Serve(l, nil)
}

func (c *Coordinator) Loop() {
	ticker := time.NewTicker(200 * time.Millisecond)
	for {
		<-ticker.C
		if c.Phase == "map" {
			done := c.CheckMapDone()
			if !done {
				continue
			}
			c.Phase = "reduce"
			c.Shuffle()
			c.SpawnReducers()
			break
		}
	}
}

func (c *Coordinator) Shuffle() {
	c.CompletedMapsMx.RLock()
	defer c.CompletedMapsMx.RUnlock()
	for i := 0; i < c.Reducers; i++ {
		bucket := strconv.Itoa(i)
		inputFiles := []string{}
		outputFile := c.Path + "output/" + bucket
		for _, uid := range c.CompletedMaps {
			path := c.Path + "intermediate/" + uid + "-" + bucket
			_, err := os.Stat(path)
			if err != nil {
				continue
			}
			inputFiles = append(inputFiles, path)
		}
		if len(inputFiles) == 0 {
			continue
		}
		uid := atomic.AddInt64(&c.TaskCounter, 1)
		task := mr.ReduceTask{
			Uid:        strconv.FormatInt(uid, 10),
			InputFiles: inputFiles,
			OutputFile: outputFile,
		}
		c.ReduceQueue = append(c.ReduceQueue, &task)
	}
}

func (c *Coordinator) CreateJob() error {
	job := struct {
		Mappers  int
		Reducers int
		Path     string
	}{
		Mappers:  4,
		Reducers: 2,
		Path:     "/files/",
	}
	c.Mappers = job.Mappers
	c.Reducers = job.Reducers
	c.Path = job.Path
	inputPath := c.Path + "input/"
	files, err := os.ReadDir(inputPath)
	if err != nil {
		return err
	}
	c.MapQueue = make([]*mr.MapTask, len(files))
	for i, file := range files {
		uid := atomic.AddInt64(&c.TaskCounter, 1)
		path := inputPath + file.Name()
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

func (c *Coordinator) SpawnMappers() {
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
			Name: "wordcount-mapper",
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

func (c *Coordinator) SpawnReducers() {
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
			Name: "wordcount-reducer",
		},
		Spec: batch.JobSpec{
			Parallelism: int32Ptr(int32(c.Reducers)),
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
								Value: "reduce",
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
	result, err := batchClient.Create(context.TODO(), job, meta.CreateOptions{})
	if err != nil {
		panic(err)
	}
	fmt.Printf("Created job %q.\n", result.GetObjectMeta().GetName())
}

func int32Ptr(i int32) *int32 { return &i }
