package mr

import (
	"log"
	"net"
	"net/http"
	"net/rpc"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Coordinator struct {
	// Your definitions here.
	ReducerNum        int            // 传入的参数决定需要多少个reducer
	TaskId            int            // 用于生成task的特殊id
	DistPhase         Phase          // 目前整个框架应该处于什么任务阶段
	TaskChannelReduce chan *Task     // 使用chan保证并发安全
	TaskChannelMap    chan *Task     // 使用chan保证并发安全
	taskMetaHolder    TaskMetaHolder // 存着task
	files             []string       // 传入的文件数组
	mu                sync.Mutex
}

// TaskMetaHolder 保存全部任务的元数据
type TaskMetaHolder struct {
	MetaMap map[int]*TaskMetaInfo // 通过下标hash快速定位
}

// TaskMetaInfo 保存任务的元数据
type TaskMetaInfo struct {
	state    State // 任务的状态
	TaskAdr  *Task // 传入任务的指针,为的是这个任务从通道中取出来后，还能通过地址标记这个任务已经完成
	TaskTime time.Time
}

// Your code here -- RPC handlers for the worker to call.

// an example RPC handler.
//
// the RPC argument and reply types are defined in rpc.go.
func (c *Coordinator) Example(args *ExampleArgs, reply *ExampleReply) error {
	reply.Y = args.X + 1
	return nil
}

// start a thread that listens for RPCs from worker.go
func (c *Coordinator) server() {
	rpc.Register(c)
	rpc.HandleHTTP()
	//l, e := net.Listen("tcp", ":1234")
	sockname := coordinatorSock()
	os.Remove(sockname)
	l, e := net.Listen("unix", sockname)
	if e != nil {
		log.Fatal("listen error:", e)
	}
	go http.Serve(l, nil)
}

// main/mrcoordinator.go calls Done() periodically to find out
// if the entire job has finished.
func (c *Coordinator) Done() bool {
	// Your code here.
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.DistPhase == AllDone {
		// log.Println("all done!!!")
		return true
	}
	return false
}

// create a Coordinator.
// main/mrcoordinator.go calls this function.
// nReduce is the number of reduce tasks to use.
func MakeCoordinator(files []string, nReduce int) *Coordinator {
	// Your code here.
	c := Coordinator{
		files:             files,
		ReducerNum:        nReduce,
		DistPhase:         MapPhase,
		TaskId:            0,
		TaskChannelMap:    make(chan *Task, len(files)),
		TaskChannelReduce: make(chan *Task, nReduce),
		taskMetaHolder: TaskMetaHolder{
			MetaMap: make(map[int]*TaskMetaInfo, len(files)+nReduce),
			// 任务的总数应该是files + Reducer的数量
		},
	}
	// 初始化map 任务
	c.makeMapTasks(files)
	go c.crash()
	c.server()
	return &c
}

func (c *Coordinator) makeMapTasks(files []string) {
	for _, file := range files {
		// 获取id
		id := c.getid()
		// 构造任务
		task := Task{
			TaskType:   MapTask,
			TaskId:     id,
			ReducerNum: c.ReducerNum,
		}
		task.Files = append(task.Files, file)
		// 构造metainfo
		meta := TaskMetaInfo{
			state:   Waiting,
			TaskAdr: &task,
		}
		// 存入任务切片
		c.taskMetaHolder.appendMeta(&meta)
		// 存入map任务队列
		c.TaskChannelMap <- &task
	}
}
func (c *Coordinator) makeReduceTasks() {
	for i := 0; i < c.ReducerNum; i++ {
		// 获取id
		id := c.getid()
		// 构造任务
		task := Task{
			TaskType: ReduceTask,
			TaskId:   id,
			Files:    c.getReduceFile(i), // 获取缓存文件
		}

		// 构造metainfo
		meta := TaskMetaInfo{
			state:   Waiting,
			TaskAdr: &task,
		}
		// 存入任务切片
		c.taskMetaHolder.appendMeta(&meta)
		// 存入map任务队列
		c.TaskChannelReduce <- &task
	}
}
func (c *Coordinator) getid() int {
	id := c.TaskId
	// id 自增
	c.TaskId++
	return id
}
func (c *Coordinator) getReduceFile(i int) []string {
	rfiles := make([]string, 0)
	dir, _ := os.Getwd()
	files, _ := os.ReadDir(dir)
	// for _, file := range files {
	// 	log.Printf("dir:%v  files:%v\n", dir, file.Name())
	// }

	for _, file := range files {
		if strings.HasPrefix(file.Name(), "mr-tmp") && strings.HasSuffix(file.Name(), strconv.Itoa(i)) {
			rfiles = append(rfiles, file.Name())
		}
	}
	return rfiles
}
func (c *TaskMetaHolder) appendMeta(meta *TaskMetaInfo) bool {
	tid := meta.TaskAdr.TaskId
	mt, ok := c.MetaMap[tid]
	if !ok || mt == nil {
		c.MetaMap[tid] = meta
	} else {
		return false
	}
	return true
}

// rpc : worker拉取任务
func (c *Coordinator) DistributeTask(args *TaskArgs, reply *Task) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 判断阶段-->>>>
	switch c.DistPhase {
	case MapPhase:
		// 判断队列中是否还有可执行任务
		if len(c.TaskChannelMap) > 0 {
			*reply = *<-c.TaskChannelMap
			// 取出来之后要判断任务的状态 严谨一波
			// 其实没必要
			tmi := c.taskMetaHolder.MetaMap[reply.TaskId]
			if tmi != nil && tmi.state == Waiting {
				tmi.state = Working
				tmi.TaskTime = time.Now()
			}
		} else {
			// 队列为空
			// 返回等待任务
			reply.TaskType = WaittingTask
			// 判断是否要进入下一阶段
			if c.taskMetaHolder.checkDone() {
				c.nextPhase()
			}
		}
	case ReducePhase:
		if len(c.TaskChannelReduce) > 0 {
			*reply = *<-c.TaskChannelReduce
			tmi := c.taskMetaHolder.MetaMap[reply.TaskId]
			if tmi != nil && tmi.state == Waiting {
				tmi.state = Working
				tmi.TaskTime = time.Now()
			}
		} else {
			// 队列为空
			// 返回等待任务
			reply.TaskType = WaittingTask
			// 判断是否要进入下一阶段
			if c.taskMetaHolder.checkDone() {
				c.nextPhase()
			}
		}
	case AllDone:
		reply.TaskType = ExitTask
	default:
		reply.TaskType = ExitTask
	}
	return nil
}

func (c *Coordinator) FinishTask(args *Task, reply *Task) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	switch args.TaskType {
	case MapTask:
		//
		tmi := c.taskMetaHolder.MetaMap[args.TaskId]
		if tmi != nil && tmi.state == Working {
			tmi.state = Done
			// log.Printf("map task %d finish", tmi.TaskAdr.TaskId)
		}
	case ReduceTask:
		tmi := c.taskMetaHolder.MetaMap[args.TaskId]
		if tmi != nil && tmi.state == Working {
			tmi.state = Done
			// log.Printf("reduce task %d finish", tmi.TaskAdr.TaskId)
		}
	default:
		// log.Println("undefine task type")
	}
	return nil
}

// 判断coordinator是否可以进入下一阶段
func (t *TaskMetaHolder) checkDone() bool {
	mapDone := 0
	mapUndone := 0
	reduceDone := 0
	reduceUndone := 0

	// 遍历metaslice
	for _, meta := range t.MetaMap {
		if meta.TaskAdr.TaskType == MapTask {
			if meta.state == Done {
				mapDone++
				continue
			}
			mapUndone++
		}
		if meta.TaskAdr.TaskType == ReduceTask {
			if meta.state == Done {
				reduceDone++
				continue
			}
			reduceUndone++
		}
	}

	if mapDone > 0 && mapUndone == 0 && (reduceDone == 0 && reduceUndone == 0) {
		return true
	} else if reduceDone > 0 && reduceUndone == 0 {
		return true
	}
	return false
}

func (c *Coordinator) nextPhase() {
	if c.DistPhase == MapPhase {
		c.DistPhase = ReducePhase
		// make reduce任务
		c.makeReduceTasks()
	} else if c.DistPhase == ReducePhase {
		c.DistPhase = AllDone
	}
}

func (c *Coordinator) crash() {
	for {
		c.mu.Lock()
		if c.DistPhase == AllDone {
			c.mu.Unlock()
			break
		}
		for _, meta := range c.taskMetaHolder.MetaMap {
			if meta.state == Working && time.Since(meta.TaskTime) > 10*time.Second {
				// 将任务重新放入队列中
				meta.state = Waiting
				if meta.TaskAdr.TaskType == MapTask {
					c.TaskChannelMap <- meta.TaskAdr
				} else if meta.TaskAdr.TaskType == ReduceTask {
					c.TaskChannelReduce <- meta.TaskAdr
				}
			}
		}
		c.mu.Unlock()
		time.Sleep(2 * time.Second)
	}
}
