package mr

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io"
	"log"
	"net/rpc"
	"os"
	"sort"
	"strconv"
	"time"
)

// Task worker向coordinator获取task的结构体
type Task struct {
	TaskType   TaskType // 任务类型判断到底是map还是reduce
	TaskId     int      // 任务的id
	ReducerNum int      // 传入的reducer的数量，用于hash
	Files      []string // 输入文件
}

// TaskArgs rpc应该传入的参数，可实际上应该什么都不用传,因为只是worker获取一个任务
type TaskArgs struct{}

// TaskType 对于下方枚举任务的父类型
type TaskType int

// Phase 对于分配任务阶段的父类型
type Phase int

// State 任务的状态的父类型
type State int

// 枚举任务的类型
const (
	MapTask TaskType = iota
	ReduceTask
	WaittingTask // Waittingen任务代表此时为任务都分发完了，但是任务还没完成，阶段未改变
	ExitTask     // exit
)

// 枚举阶段的类型
const (
	MapPhase    Phase = iota // 此阶段在分发MapTask
	ReducePhase              // 此阶段在分发ReduceTask
	AllDone                  // 此阶段已完成
)

// 任务状态类型
const (
	Working State = iota // 此阶段在工作
	Waiting              // 此阶段在等待执行
	Done                 // 此阶段已经做完
)

// Map functions return a slice of KeyValue.
type KeyValue struct {
	Key   string
	Value string
}
type SortedKey []KeyValue

// use ihash(key) % NReduce to choose the reduce
// task number for each KeyValue emitted by Map.
func ihash(key string) int {
	h := fnv.New32a()
	h.Write([]byte(key))
	return int(h.Sum32() & 0x7fffffff)
}

// main/mrworker.go calls this function.
func Worker(mapf func(string, string) []KeyValue,
	reducef func(string, []string) string) {

	// Your worker implementation here.
	keeplive := true
	for keeplive { // while
		// rpc调用获取任务  coordinator枷锁
		task := GetTask()
		// 根据任务类型执行函数
		switch task.TaskType {
		case MapTask:
			DealMap(mapf, &task)
			callDone(&task)
		case ReduceTask:
			DealReduce(reducef, &task)
			callDone(&task)
		case WaittingTask:
			time.Sleep(time.Second)
		case ExitTask:
			keeplive = false
		}
		// 根据返回状态决定是否继续
	}
	// uncomment to send the Example RPC to the coordinator.
	// CallExample()

}

// example function to show how to make an RPC call to the coordinator.
//
// the RPC argument and reply types are defined in rpc.go.
func CallExample() {

	// declare an argument structure.
	args := ExampleArgs{}

	// fill in the argument(s).
	args.X = 99

	// declare a reply structure.
	reply := ExampleReply{}

	// send the RPC request, wait for the reply.
	// the "Coordinator.Example" tells the
	// receiving server that we'd like to call
	// the Example() method of struct Coordinator.
	ok := call("Coordinator.Example", &args, &reply)
	if ok {
		// reply.Y should be 100.
		fmt.Printf("reply.Y %v\n", reply.Y)
	} else {
		fmt.Printf("call failed!\n")
	}
}

// send an RPC request to the coordinator, wait for the response.
// usually returns true.
// returns false if something goes wrong.
func call(rpcname string, args interface{}, reply interface{}) bool {
	// c, err := rpc.DialHTTP("tcp", "127.0.0.1"+":1234")
	sockname := coordinatorSock()
	c, err := rpc.DialHTTP("unix", sockname)
	if err != nil {
		log.Fatal("dialing:", err)
	}
	defer c.Close()

	err = c.Call(rpcname, args, reply)
	if err == nil {
		return true
	}

	fmt.Println(err)
	return false
}

// GetTask 获取任务
func GetTask() Task {

	args := TaskArgs{}
	reply := Task{}
	ok := call("Coordinator.DistributeTask", &args, &reply)
	if ok {
		// log.Println("DistributeTask:", reply.TaskId, "--", reply.TaskType)
	} else {
		// log.Println("call failed!")
	}
	return reply

}

// 处理map任务
func DealMap(mapf func(string, string) []KeyValue, map_task *Task) {
	// 将文件中内容读取出来
	for _, filename := range map_task.Files {

		file, err := os.Open(filename)
		if err != nil {
			log.Fatalln("file open error:", err)
		}
		content, err := io.ReadAll(file)
		if err != nil {
			log.Fatalln("file read error:", err)
		}
		file.Close()

		// 将内容和filename交给mapf处理
		// 返回一组kv值
		kvs := mapf(filename, string(content))

		// 将kv值分成r份
		rn := map_task.ReducerNum
		kvRN := make([][]KeyValue, rn)

		for _, kv := range kvs {
			kvRN[ihash(kv.Key)%rn] = append(kvRN[ihash(kv.Key)%rn], kv)
		}
		// 输出到文件 --- 也就是paper中提到的内存缓存区
		// 或者可以先创建文件组然后将内容写入
		for i := 0; i < rn; i++ {
			output_name := "mr-tmp-" + strconv.Itoa(map_task.TaskId) + "-" + strconv.Itoa(i)
			ofile, _ := os.Create(output_name)
			encoder := json.NewEncoder(ofile)
			for _, kv := range kvRN[i] {
				encoder.Encode(kv)
			}
			ofile.Close()
		}
	}
}

// callDone Call RPC to mark the task as completed
func callDone(args *Task) Task {

	reply := Task{}
	ok := call("Coordinator.FinishTask", args, &reply)
	if ok {
		// log.Println("FinishTask:", reply.TaskId, "--", reply.TaskType)
	} else {
		// log.Printf("callDone failed!\n")
	}
	return reply

}

func DealReduce(reducef func(string, []string) string, task *Task) {
	// 一个reduce任务输出一个文件
	reduceFileNum := task.TaskId
	// log.Printf("deal reduce files--:%v\n", task.Files)
	intermediate := shuffle(task.Files)
	dir, _ := os.Getwd()
	//tempFile, err := ioutil.TempFile(dir, "mr-tmp-*")
	tempFile, err := os.CreateTemp(dir, "mr-tmp-*")
	if err != nil {
		log.Fatal("Failed to create temp file", err)
	}
	i := 0
	for i < len(intermediate) {
		j := i + 1
		for j < len(intermediate) && intermediate[j].Key == intermediate[i].Key {
			j++
		}
		var values []string
		for k := i; k < j; k++ { // 同一个key的不同value聚合在一个数组
			values = append(values, intermediate[k].Value)
		}
		output := reducef(intermediate[i].Key, values)                // 最后reducef聚合成一对键值对
		fmt.Fprintf(tempFile, "%v %v\n", intermediate[i].Key, output) // 输出到文件
		i = j
	}
	tempFile.Close()
	fn := fmt.Sprintf("mr-out-%d", reduceFileNum)
	os.Rename(tempFile.Name(), fn)
}

// 洗牌方法，得到一组排序好的kv数组
func shuffle(files []string) []KeyValue {
	var kva []KeyValue
	for _, filepath := range files {
		file, _ := os.Open(filepath)
		dec := json.NewDecoder(file)
		for {
			var kv KeyValue
			if err := dec.Decode(&kv); err != nil {
				break
			}
			kva = append(kva, kv)
		}
		file.Close()
	}
	sort.Sort(SortedKey(kva))
	return kva
}
func (s SortedKey) Len() int {
	return len(s)
}
func (s SortedKey) Less(i, j int) bool {
	return s[i].Key < s[j].Key
}
func (s SortedKey) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}
