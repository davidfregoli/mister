package mister

import (
	"errors"
	"fmt"
	"log"
	"net/rpc"
	"os"
	"strconv"
	"time"
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
	partitions, err := strconv.Atoi(os.Getenv("MISTER_PARTITIONS"))
	if err != nil {
		log.Fatal("Cannot parse MISTER_PARTITIONS environment variable: ", err)
	}
	worker := NewWorker(app, phase, podname, partitions)
	worker.Run()
	// if err != nil {
	// 	panic(err)
	// }
	// worker.Start()
	// fmt.Println(mapped)
	// CallSendJob()
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

// send an RPC request to the coordinator, wait for the response.
// usually returns true.
// returns false if something goes wrong.
func call(rpcname string, args interface{}, reply interface{}) bool {
	time.Sleep(time.Second * 5)
	c, err := rpc.DialHTTP("tcp", "coordinator:1234")
	if err != nil {
		fmt.Println("Server shutdown or unreachable.")
		os.Exit(0)
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
