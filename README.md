# MIT6.824Lab



![lab2-guide](C:\Users\romeo\Pictures\GoMoe\lab2-guide.png)



## LAB2

### raft：

#### 	2A：

​	2a任务主要是选举，看某一个节点是否能够连任leader

流程图大致如下：

Pa:

![lab2-main](C:\Users\romeo\Pictures\GoMoe\lab2-main.png)

eg:Pa是主流程图，不包含rpc内容

Pb:

![lab2-reqvote](C:\Users\romeo\Pictures\GoMoe\lab2-reqvote.png)

eg:reqvote请求流程



### 其他问题杂谈：

- 锁问题

  ![lab2-lock](C:\Users\romeo\Pictures\GoMoe\lab2-lock.png)

- timer问题

  心跳时间120ms

  初始过期时间为150-350ms

  reset过期时间为200-400ms	
