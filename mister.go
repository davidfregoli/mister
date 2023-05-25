package mister

import (
	"errors"
	"fmt"
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

type Job struct {
	MCount int
	RCount int
	Path   string
}

type SendJobArgs struct {
	Job
}
type SendJobReply struct {
	Uid int
}
type GetJobMapsReply struct {
	Maps []MapTask
}

func CallSendJob() error {
	// declare an argument structure.
	args := SendJobArgs{Job{MCount: 10, RCount: 5, Path: "/mister"}}
	// declare a reply structure.
	reply := SendJobReply{}

	ok := call("Coordinator.SendJob", &args, &reply)
	if !ok {
		return errors.New("cannot send completed tasks to server")
	}
	fmt.Println("rpc sent. uid: " + strconv.Itoa(reply.Uid))
	return nil
}

func CallGetJobMaps() ([]MapTask, error) {
	args := Stub{}
	reply := GetJobMapsReply{}
	ok := call("Coordinator.GetJobMaps", &args, &reply)
	if !ok {
		return nil, errors.New("cannot get job maps")
	}
	return reply.Maps, nil
}

// send an RPC request to the coordinator, wait for the response.
// usually returns true.
// returns false if something goes wrong.
func call(rpcname string, args interface{}, reply interface{}) bool {
	c, err := rpc.DialHTTP("tcp", "coordinator:1234")
	if err != nil {
		fmt.Println("Server shutdown or unreachable.")
		return false
	}
	defer c.Close()

	err = c.Call(rpcname, args, reply)
	if err == nil {
		return true
	}

	fmt.Println(err)
	return false
}

func Int32Ptr(i int32) *int32 { return &i }
