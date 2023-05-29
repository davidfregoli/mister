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
	TaskCounter      int64
	Path             string
	Phase            string
	Mappers          int
	Reducers         int
	MapTasks         []mr.MapTask
	MapTasksMx       sync.RWMutex
	MapQueue         chan *mr.MapTask
	MapRunners       sync.Map
	CompletedMaps    int64
	ReduceTasks      []mr.ReduceTask
	ReduceTasksMx    sync.RWMutex
	ReduceQueue      chan *mr.ReduceTask
	ReduceRunners    sync.Map
	CompletedReduces int64
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
	c.MapQueue = make(chan *mr.MapTask, len(files))
	c.MapTasks = make([]mr.MapTask, len(files))
	for i, file := range files {
		uid := atomic.AddInt64(&c.TaskCounter, 1)
		path := inputPath + file.Name()
		if err != nil {
			return err
		}
		c.MapTasks[i] = mr.MapTask{
			Uid:       strconv.FormatInt(uid, 10),
			InputFile: path,
			Status:    "idle",
		}
		c.MapQueue <- &c.MapTasks[i]
	}
	return nil
}

func (c *Coordinator) GetJobMaps(args *mr.Stub, reply *mr.GetJobMapsReply) error {
	c.MapTasksMx.RLock()
	reply.Maps = c.MapTasks
	c.MapTasksMx.RUnlock()
	return nil
}

func (c *Coordinator) GetMapTask(args *mr.GetMapTaskArgs, reply *mr.GetMapTaskReply) error {
	if len(c.MapQueue) == 0 {
		reply.Done = int(c.CompletedMaps) == len(c.MapTasks)
		return nil
	}
	task := <-c.MapQueue
	c.MapTasksMx.Lock()
	task.Status = "running"
	task.Reducers = c.Reducers
	task.Worker = args.Worker
	c.MapTasksMx.Unlock()
	c.MapRunners.Store(args.Worker, task)
	reply.Found = true
	reply.MapTask = *task

	log.Println("Assigning Map task " + task.Uid + " to worker " + task.Worker)
	return nil
}

func (c *Coordinator) GetReduceTask(args *mr.GetReduceTaskArgs, reply *mr.GetReduceTaskReply) error {
	if len(c.ReduceQueue) == 0 {
		reply.Done = int(c.CompletedReduces) == len(c.ReduceTasks)
		return nil
	}
	task := <-c.ReduceQueue
	c.ReduceTasksMx.Lock()
	task.Status = "running"
	task.Worker = args.Worker
	c.ReduceTasksMx.Unlock()
	c.ReduceRunners.Store(args.Worker, task)
	reply.Found = true
	reply.ReduceTask = *task

	log.Println("Assigning Reduce task " + task.Uid + " to worker " + task.Worker)
	return nil
}

func (c *Coordinator) NotifyCompletedMap(args *mr.NotifyCompoletedArgs, reply *mr.Stub) error {
	worker := args.Worker
	anyTask, loaded := c.MapRunners.LoadAndDelete(worker)
	if !loaded {
		log.Fatal("cannot retrieve task for worker " + worker)
	}
	task := anyTask.(*mr.MapTask)
	c.MapTasksMx.Lock()
	task.Status = "completed"
	c.MapTasksMx.Unlock()
	atomic.AddInt64(&c.CompletedMaps, 1)
	return nil
}

func (c *Coordinator) NotifyCompletedReduce(args *mr.NotifyCompoletedArgs, reply *mr.Stub) error {
	worker := args.Worker
	anyTask, loaded := c.ReduceRunners.LoadAndDelete(worker)
	if !loaded {
		log.Fatal("cannot retrieve task for worker " + worker)
	}
	task := anyTask.(*mr.ReduceTask)
	c.ReduceTasksMx.Lock()
	task.Status = "completed"
	c.ReduceTasksMx.Unlock()
	atomic.AddInt64(&c.CompletedReduces, 1)
	return nil
}

func (c *Coordinator) CheckMapDone() bool {
	return int(c.CompletedMaps) == len(c.MapTasks)
}

func (c *Coordinator) CheckReduceDone() bool {
	return int(c.CompletedReduces) == len(c.ReduceTasks)
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
		} else if c.Phase == "reduce" {
			done := c.CheckReduceDone()
			if !done {
				continue
			}
			c.Phase = "idle"
			log.Println(("job completed"))
			break
		}
	}
}

func (c *Coordinator) Shuffle() {
	log.Println("shufflin")
	c.ReduceTasks = make([]mr.ReduceTask, c.Reducers)
	c.ReduceQueue = make(chan *mr.ReduceTask, c.Reducers)
	for i := 0; i < c.Reducers; i++ {
		bucket := strconv.Itoa(i)
		inputFiles := []string{}
		outputFile := c.Path + "output/" + bucket
		for _, task := range c.MapTasks {
			path := c.Path + "intermediate/" + task.Uid + "-" + bucket
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
		c.ReduceTasks[i] = mr.ReduceTask{
			Uid:        strconv.FormatInt(uid, 10),
			InputFiles: inputFiles,
			OutputFile: outputFile,
		}
		c.ReduceQueue <- &c.ReduceTasks[i]
	}
}

func (c *Coordinator) SpawnMappers() {
	var config *rest.Config
	for {
		var err error
		// creates the in-cluster config
		config, err = rest.InClusterConfig()
		if err != nil {
			time.Sleep(time.Second)
		} else {
			break
		}
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
