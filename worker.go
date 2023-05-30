package mister

import (
	"bufio"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"
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
		fmt.Println(task, found, done, err)
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
		time.Sleep(time.Second)
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
	worker.CallNotifyCompletedMap(task.Uid)
}

func (worker *Worker) RunReduceLoop() {
	task, found, done, err := worker.CallGetReduceTask()
	fmt.Println(task, found, done, err)
	if err != nil {
		log.Fatal(err)
	}
	worker.RunReduceTask(task)
}

func (worker *Worker) RunReduceTask(task ReduceTask) {
	inter := []KeyValue{}

	for _, file := range task.InputFiles {
		file, err := os.Open(file)
		if err != nil {
			log.Fatal(err)
		}

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			kvarr := strings.Split(scanner.Text(), " ")
			kv := KeyValue{Key: kvarr[0], Value: kvarr[1]}
			inter = append(inter, kv)
		}
		file.Close()

		err = scanner.Err()
		if err != nil {
			log.Fatal(err)
		}
	}

	sort.Slice(inter, func(i, j int) bool {
		return inter[i].Key < inter[j].Key
	})
	outputFile, err := os.Create(task.OutputFile)
	if err != nil {
		log.Fatal(err)
	}
	for i, j := 0, 0; i < len(inter); i = j {
		j = i + 1
		for j < len(inter) && inter[j].Key == inter[i].Key {
			j++
		}
		values := []string{}
		for k := i; k < j; k++ {
			values = append(values, inter[k].Value)
		}
		output := worker.App.Reduce(inter[i].Key, values)

		fmt.Fprintf(outputFile, "%v %v\n", inter[i].Key, output)
	}
	outputFile.Close()
	worker.CallNotifyCompletedReduce(task.Uid)
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

func (worker *Worker) CallGetReduceTask() (ReduceTask, bool, bool, error) {
	args := GetReduceTaskArgs{Worker: worker.Name}
	reply := GetReduceTaskReply{}

	ok := call("Coordinator.GetReduceTask", &args, &reply)
	if !ok {
		return ReduceTask{}, false, false, errors.New("cannot get reduce task from server")
	}
	return reply.ReduceTask, reply.Found, reply.Done, nil
}

func (worker *Worker) CallNotifyCompletedMap(uid string) {
	args := NotifyCompoletedArgs{Uid: uid, Worker: worker.Name}
	reply := &Stub{}
	fmt.Println("notifying ", uid)

	ok := call("Coordinator.NotifyCompletedMap", &args, &reply)
	if !ok {
		log.Fatal("could not notify completion")
	}
}

func (worker *Worker) CallNotifyCompletedReduce(uid string) {
	args := NotifyCompoletedArgs{Uid: uid, Worker: worker.Name}
	reply := &Stub{}
	fmt.Println("notifying ", uid)

	ok := call("Coordinator.NotifyCompletedReduce", &args, &reply)
	if !ok {
		log.Fatal("could not notify completion")
	}
}

type MapTask struct {
	InputFile string
	Reducers  int
	Uid       string
	Worker    string
	Status    string
}

type ReduceTask struct {
	Uid        string
	InputFiles []string
	OutputFile string
	Worker     string
	Status     string
}

type NotifyCompoletedArgs struct {
	Uid    string
	Worker string
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

type GetReduceTaskArgs struct {
	Worker string
}

type GetReduceTaskReply struct {
	ReduceTask
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
