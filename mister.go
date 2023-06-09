package mister

import (
	"errors"
	"log"
	"net/rpc"
	"os"
	"strconv"
)

type KeyValue struct {
	Key   string
	Value string
}

type App interface {
	Map(filename string, contents string) []KeyValue
	Reduce(key string, values []string) string
}

func RegisterApp(app App) {
	phase := os.Getenv("MISTER_WORKER_PHASE")
	podname := os.Getenv("MISTER_POD_NAME")
	reducers, err := strconv.Atoi(os.Getenv("MISTER_REDUCERS"))
	if phase == "map" && err != nil {
		log.Fatal("Cannot parse MISTER_REDUCERS environment variable: ", err, phase)
	}
	worker := NewWorker(app, phase, podname, reducers)
	worker.Run()
}

type GetJobReply struct {
	Maps    []MapTask
	Reduces []ReduceTask
	Done    bool
}

func CallGetJob() (GetJobReply, error) {
	args := Stub{}
	reply := GetJobReply{}
	ok := call("Coordinator.GetJob", &args, &reply)
	if !ok {
		return GetJobReply{}, errors.New("cannot get job maps")
	}
	return reply, nil
}

type MakeJobArgs struct {
	Mappers  int
	Reducers int
	Path     string
	App      string
}

func CallMakeJob(app string, mappers int, reducers int) error {
	args := MakeJobArgs{
		App:      app,
		Mappers:  mappers,
		Reducers: reducers,
		Path:     "/files/",
	}
	reply := Stub{}
	go call("Coordinator.MakeJob", &args, &reply)
	return nil
}

// send an RPC request to the coordinator, wait for the response.
// usually returns true.
// returns false if something goes wrong.
func call(rpcname string, args interface{}, reply interface{}) bool {
	c, err := rpc.DialHTTP("tcp", "coordinator:1234")
	if err != nil {
		log.Println("Server shutdown or unreachable.")
		return false
	}
	defer c.Close()

	err = c.Call(rpcname, args, reply)
	if err == nil {
		return true
	}

	log.Println(err)
	return false
}

func Int32Ptr(i int32) *int32 { return &i }
