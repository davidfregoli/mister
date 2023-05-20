package mister

import (
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"log"
	"os"
	"strconv"
	"time"
)

type Worker struct {
	App      App
	Name     string
	Reducers int
	Phase    string
}

func NewWorker(app App, phase string, podname string, reducers int) Worker {
	return Worker{
		App:      app,
		Name:     podname,
		Reducers: reducers,
		Phase:    phase,
	}
}

func (worker *Worker) Run() {
	if worker.Phase == "map" {
		worker.RunMapLoop()
	} else if worker.Phase == "reduce" {
		worker.RunReduceLoop()
	}
}

func (worker *Worker) RunMapLoop() {
	for {
		task, found, done, err := worker.CallGetMapTask()
		if err != nil {
			fmt.Println(err)
			continue
		}
		if done {
			break
		}
		if !found {
			time.Sleep(time.Second)
			continue
		}
		worker.RunMapTask(task)
	}
}

func (worker *Worker) RunMapTask(task MapTask) {
	input := readFile(task.InputFile)
	intermediates := map[int][]KeyValue{}
	kva := worker.App.Map(task.InputFile, input)
	for _, entry := range kva {
		bucket := spread(entry.Key, task.Reducers)
		intermediates[bucket] = append(intermediates[bucket], entry)
	}
	for bucket, data := range intermediates {
		iname := "/files/intermediate/" + task.Uid + "-" + strconv.Itoa(bucket)
		ifile, _ := os.Create(iname)
		for _, entry := range data {
			fmt.Fprintf(ifile, "%v %v\n", entry.Key, entry.Value)
		}
		ifile.Close()
	}
	worker.CallNotifyCompleted(task.Uid)
}

func (worker *Worker) RunReduceLoop() {
}

func (worker *Worker) CallGetMapTask() (MapTask, bool, bool, error) {
	args := GetMapTaskArgs{Worker: worker.Name}
	reply := GetMapTaskReply{}

	ok := call("Coordinator.GetMapTask", &args, &reply)
	if !ok {
		return MapTask{}, false, false, errors.New("cannot get map task from server")
	}
	return reply.MapTask, reply.Found, reply.Done, nil
}

func (worker *Worker) CallNotifyCompleted(uid string) {
	args := NotifyCompoletedArgs{Uid: uid}
	reply := &Stub{}

	ok := call("Coordinator.NotifyCompleted", &args, &reply)
	if !ok {
		log.Fatal("could not notify completion")
	}
}

type MapTask struct {
	Uid       string
	InputFile string
	Reducers  int
	Worker    string
	Started   int64
}

type NotifyCompoletedArgs struct {
	Uid string
}

type Stub struct{}

type GetMapTaskArgs struct {
	Worker string
}

type GetMapTaskReply struct {
	MapTask
	Done  bool
	Found bool
}

func readFile(filename string) string {
	input, err := os.Open(filename)
	if err != nil {
		log.Fatalf("cannot open %v", filename)
	}
	content, err := io.ReadAll(input)
	input.Close()
	if err != nil {
		log.Fatalf("cannot read %v", filename)
	}
	return string(content)
}

func ihash(key string) int {
	h := fnv.New32a()
	h.Write([]byte(key))
	return int(h.Sum32() & 0x7fffffff)
}

func spread(key string, reducers int) int {
	return ihash(key) % reducers
}
