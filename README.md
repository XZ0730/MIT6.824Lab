# MIT6.824Lab

## LAB1

### mapreduce:

​		![lab1](https://github.com/XZ0730/MIT6.824Lab/assets/94458213/698fd86a-b4bb-41db-9d28-e9ec4cd19c85)


figure1:  这边做一个对这张图的解释--->

##### 	coordinator

​	首先你要启动一个coordinator，协调者的角色程序，那么他的功能是啥呢？ 

​			☞协调任务的分发

​			☞管理任务的动态，状态

​			☞控制整个mapreduce流程的进行，或者说是推进流程进行

​	那么在做这个实验之前，需要有个对这个lab宏观的流程印象

​	首先coordinator启动，状态从map开始，这个阶段需要给worker分发map任务，说是分发，但是其实是worker进行主动请求来获取任务，coordinator在每次任务无法分发（任务队列为空）的时候会进行判断是否开始下个阶段的操作，如果成功则会进行下阶段操作

​	reduce同上，直到变成阶段状态alldone，worker取任务时通知其退出

![lab1-phase](https://github.com/XZ0730/MIT6.824Lab/assets/94458213/c147dcf4-5493-49fe-9f95-f3faedd337b9)


​	然后就是worker，worker的功能是去执行具体的map/reduce的逻辑，worker其实无差别，真正的差别取决于他获取到的任务的类型，根据任务类型执行不同的逻辑，paper中提到的除了map，reduce两种任务类型，还可以增加两种，一种是wait任务类型，一种是exit任务类型，这个下面另做解释。

##### 	map任务worker需要做什么？

​	worker需要把从coordinator获取的任务中的files 每个单独进行map操作(将内容进行提炼)，每次返回一组kv组，在这个lab中也就是key: word  value: 1 这种类型的kv对，返回的是一组也就是文件中的所有行都被构造成这样的一个对象，然后放入切片中返回构成一组kv。

​	这组kv再根据每个元素的key 做ihash操作，取余nReduce(一开始构造coordinaotor的时候一起给定的超参数)，这样得到的值是一个tmp文件的后缀，这个后缀是为了分类kv，你需要根据得到的不同的后缀，将kv输出到不同的tmp文件中，其实也就是缓存？中间文件？paper中提到是说放在缓存中，放入文件中只是一种实现，但最终目的是为了reduce任务进行服务。

​		---------简单来说就是把原来的文件中kv提取出来再根据key分类，放入不同的文件，我们的最终目的是统计key出现的次数，这样可以进行有效的分类

​	然后就会生成多个tmp文件，也就是图中那个intermediate file

##### 	reduce任务需要做什么？

​	reduce任务中会包含一批ihash%nReduce相同的文件，构造任务时会根据相同nReduce后缀的一批文件放入一个任务中，等待reduce worker处理。

​	reduce任务会从这一批文件中提取kv值，构造成切片，然后进行shuffle排序，按照key编码大小。

​	然后进行统计同key的kv，reducef 函数去重，放入out文件得到最终的结果

```Go
for i < len(intermediate) {
		j := i + 1
		for j < len(intermediate) && intermediate[j].Key == intermediate[i].Key {
			j++
		}
		var values []string
		for k := i; k < j; k++ { // 同一个key的不同value聚合在一个数组
			values = append(values, intermediate[k].Value)
		}
		output := reducef(intermediate[i].Key, values)   // 最后reducef聚合成一对键值对
		fmt.Fprintf(tempFile, "%v %v\n", intermediate[i].Key, output) // 输出到文件
		i = j
	}
```

#### 补充

##### 任务类型

​	当worker收到wait任务类型，会等待两秒或者？秒再去获取任务

​	当worker收到exit任务类型，就是所有任务已经完成，可以退出了

​	上述两种任务类型其实是对应两种不同的场景进行的，这个再paper中有提到

​	一种是任务分发完，但是还没执行完，此时请求分发任务，应该让已经做完的worker等待进行下阶段操作

​	一种是worker需要退出，如何进行退出，退出的方式，而exit任务就是一种实现

##### 文件说明

​	左边有个input files输入的文件， 可以是一个或者多个，论文给的例子是你用爬虫爬取了10000...+的url数据放在一个文件中，你需要做一个分类进行reduce，去重，将相同的url给合并，由于你爬取的所有url放在一个文件中，这个时候需要进行对文件的分割，分成多个小文件，又多个worker进行并发操作，因为这个是实验，已经给好了文件的内容，文件内容的格式要求，输出要求，具体的可以看paper中的 hint要求

​	reduce中的是多个文件，map可以多个也可以单个，reduce多个是因为一开始声明的nReduce的值不为1，reduce是获取到key hash操作%nReduce相同的一批文件

​	intermediate文件命名mr-tmp-taskId-ihash(key)%nReduce

​	最终文件命名为mr-out-taskId		   命名不建议随便改，建议按规范来

#### 几个问题

​	第一个你文件名字最好别乱改，否则可能会找不到文件

​	第二个你每次任务做完需要再一次通知coordiantor这个任务做完了，所以你可能得写一个rpc函数执行这种操作，我记得仓库中的code实现是finshtask这个函数

​	**第三个bash的版本升到5.x.x版本，否则可能会导致early_exit测试无法通过**

​	第四个crash测试如何通过？ 

​	crash其实就是让你的任务一直被阻塞再worker中，使得你任务无法执行而且其他worker获取不到，这个时候你就需要做一个**定时处理 将超时的任务重新塞入任务队列**，这样worker就能再次获取到了，同时你可能要考虑一下幂等性的问题，就是如果这个worker又回来了，继续执行任务，是否会导致操作重复结果重复

​	---具体可以看代码中coordinator的crash函数实现	

​	还有，操作文件的时候用os，不要用ioutil，ioutil已经out了，不过实验中还有些内置的函数还是用的ioutil

​	还有一件事，你的**rpc函数定义要参考example**，code中有内置，不要自己定义一个缺斤少两的函数，否则无法被识别

```Go
func (c *Coordinator) Example(args *ExampleArgs, reply *ExampleReply) error {
	reply.Y = args.X + 1
	return nil
}
```



## LAB2

### raft：

![lab2-guide](https://github.com/XZ0730/MIT6.824Lab/assets/94458213/b2ac5bf1-1346-4d14-83b2-174a3365e20c)

#### 	2A：

​	2a任务主要是选举，看某一个节点是否能够连任leader

流程图大致如下：

Pa:


![lab2-main](https://github.com/XZ0730/MIT6.824Lab/assets/94458213/d8b7b8d9-8568-42ac-a8d0-592609181f6d)

eg:Pa是主流程图，不包含rpc内容

Pb:


![lab2-reqvote](https://github.com/XZ0730/MIT6.824Lab/assets/94458213/7df5592d-9dbe-4733-a056-9f9b2ddafa28)

eg:reqvote请求流程



### 其他问题杂谈：

- 锁问题


![lab2-lock](https://github.com/XZ0730/MIT6.824Lab/assets/94458213/d9eeab52-22c6-43cb-84c7-34ecd44d0854)

- timer问题

  心跳时间120ms

  初始过期时间为150-350ms

  reset过期时间为200-400ms	
