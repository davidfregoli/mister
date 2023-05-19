package mister

import (
	"errors"
	"fmt"
	"hash/fnv"
	"io/ioutil"
	"log"
	"os"
	"strconv"
	"time"
)

type Worker struct {
	App        App
	Name       string
	Partitions int
	Phase      string
}

func NewWorker(app App, phase string, podname string, partitions int) Worker {
	return Worker{
		App:        app,
		Name:       podname,
		Partitions: partitions,
		Phase:      phase,
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
		task, found, done, err := CallGetMapTask()
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
	fmt.Println(input)
	intermediates := map[int][]KeyValue{}
	kva := worker.App.Map(task.InputFile, input)
	for _, entry := range kva {
		bucket := spread(entry.Key, task.SpreadOver)
		intermediates[bucket] = append(intermediates[bucket], entry)
	}
	for bucket, data := range intermediates {
		iname := "/files/inter-" + task.Uid + "-" + strconv.Itoa(bucket)
		ifile, _ := os.Create(iname)
		for _, entry := range data {
			fmt.Fprintf(ifile, "%v %v\n", entry.Key, entry.Value)
		}
		ifile.Close()
	}
	// CallNotifyCompleted(task.Uid)

}

func (worker *Worker) RunReduceLoop() {
}

func CallGetMapTask() (MapTask, bool, bool, error) {
	// declare an argument structure.
	args := struct{}{}
	// declare a reply structure.
	reply := GetMapTaskReply{}

	ok := call("Coordinator.GetMapTask", &args, &reply)
	if !ok {
		return MapTask{}, false, false, errors.New("cannot get map task from server")
	}
	return reply.MapTask, reply.Found, reply.Done, nil
}

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

func readFile(filename string) string {
	input, err := os.Open(filename)
	if err != nil {
		log.Fatalf("cannot open %v", filename)
	}
	content, err := ioutil.ReadAll(input)
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
