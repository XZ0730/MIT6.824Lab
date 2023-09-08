# MIT6.824Lab




![lab2-guide](https://github.com/XZ0730/MIT6.824Lab/assets/94458213/b2ac5bf1-1346-4d14-83b2-174a3365e20c)



## LAB2

### raft：

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
