package raft

//
// this is an outline of the API that raft must expose to
// the service (or tester). see comments below for
// each of these functions for more details.
//
// rf = Make(...)
//   create a new Raft server.
// rf.Start(command interface{}) (index, term, isleader)
//   start agreement on a new log entry
// rf.GetState() (term, isLeader)
//   ask a Raft for its current term, and whether it thinks it is leader
// ApplyMsg
//   each time a new entry is committed to the log, each Raft peer
//   should send an ApplyMsg to the service (or tester)
//   in the same server.
//

import (
	//	"bytes"

	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	//	"6.5840/labgob"
	"lab2/labrpc"
)

// Status 节点的角色
type Status int

// VoteState 投票的状态 2A
type VoteState int

// AppendEntriesState 追加日志的状态 2A 2B
type AppendEntriesState int

// HeartBeatTimeout 定义一个全局心跳超时时间
var HeartBeatTimeout = 120 * time.Millisecond

// 枚举节点的类型：跟随者、竞选者、领导者
const (
	Follower Status = iota
	Candidate
	Leader
)

const (
	Normal VoteState = iota //投票过程正常
	Killed                  //Raft节点已终止
	Expire                  //投票(消息\竞选者）过期
	Voted                   //本Term内已经投过票

)

const (
	AppNormal    AppendEntriesState = iota // 追加正常
	AppOutOfDate                           // 追加过时
	AppKilled                              // Raft程序终止
	AppRepeat                              // 追加重复 (2B
	AppCommited                            // 追加的日志已经提交 (2B
	Mismatch                               // 追加不匹配 (2B

)

// as each Raft peer becomes aware that successive log entries are
// committed, the peer should send an ApplyMsg to the service (or
// tester) on the same server, via the applyCh passed to Make(). set
// CommandValid to true to indicate that the ApplyMsg contains a newly
// committed log entry.
//
// in part 2D you'll want to send other kinds of messages (e.g.,
// snapshots) on the applyCh, but set CommandValid to false for these
// other uses.
type ApplyMsg struct {
	CommandValid bool
	Command      interface{}
	CommandIndex int

	// For 2D:
	SnapshotValid bool
	Snapshot      []byte
	SnapshotTerm  int
	SnapshotIndex int
}
type LogEntry struct {
	Term int
}

// A Go object implementing a single Raft peer.
type Raft struct {
	mu        sync.Mutex          // Lock to protect shared access to this peer's state
	peers     []*labrpc.ClientEnd // RPC end points of all peers
	persister *Persister          // Object to hold this peer's persisted state
	me        int                 // this peer's index into peers[]
	dead      int32               // set by Kill()

	// Your data here (2A, 2B, 2C).
	// Look at the paper's Figure 2 for a description of what
	// state a Raft server must maintain.

	// 所有的servers拥有的变量:
	currentTerm int        // 记录当前的任期
	votedFor    int        // 记录当前的任期把票投给了谁
	logs        []LogEntry // 日志条目数组，包含了状态机要执行的指令集，以及收到领导时的任期号

	// 所有的servers经常修改的:
	// 正常情况下commitIndex与lastApplied应该是一样的，但是如果有一个新的提交，并且还未应用的话last应该要更小些
	commitIndex int // 状态机中已知的被提交的日志条目的索引值(初始化为0，持续递增）
	lastApplied int // 最后一个被追加到状态机日志的索引值

	// leader拥有的可见变量，用来管理他的follower(leader经常修改的）
	// nextIndex与matchIndex初始化长度应该为len(peers)，Leader对于每个Follower都记录他的nextIndex和matchIndex
	// nextIndex指的是下一个的appendEntries要从哪里开始
	// matchIndex指的是已知的某follower的log与leader的log最大匹配到第几个Index,已经apply
	nextIndex  []int // 对于每一个server，需要发送给他下一个日志条目的索引值（初始化为leader日志index+1,那么范围就对标len）
	matchIndex []int // 对于每一个server，已经复制给该server的最后日志条目下标

	// 由自己追加的:
	status   Status        // 该节点是什么角色（状态）
	overtime time.Duration // 设置超时时间，200-400ms
	timer    *time.Ticker  // 每个节点中的计时器

	applyChan chan ApplyMsg // 日志都是存在这里client取（2B）
}

// return CurrentTerm and whether this server
// believes it is the leader.
func (rf *Raft) GetState() (int, bool) {

	rf.mu.Lock()
	defer rf.mu.Unlock()
	var term int
	var isleader bool
	// Your code here (2A).
	// 一开始不知道这里还有个函数要写，这个test为了检测是否有leader
	term = rf.currentTerm
	fmt.Println("the peer[", rf.me, "] state is:", rf.status)
	if rf.status == Leader {
		isleader = true
	} else {
		isleader = false
	}
	return term, isleader
}

// save Raft's persistent state to stable storage,
// where it can later be retrieved after a crash and restart.
// see paper's Figure 2 for a description of what should be persistent.
// before you've implemented snapshots, you should pass nil as the
// second argument to persister.Save().
// after you've implemented snapshots, pass the current snapshot
// (or nil if there's not yet a snapshot).
func (rf *Raft) persist() {
	// Your code here (2C).
	// Example:
	// w := new(bytes.Buffer)
	// e := labgob.NewEncoder(w)
	// e.Encode(rf.xxx)
	// e.Encode(rf.yyy)
	// raftstate := w.Bytes()
	// rf.persister.Save(raftstate, nil)
}

// restore previously persisted state.
func (rf *Raft) readPersist(data []byte) {
	if data == nil || len(data) < 1 { // bootstrap without any state?
		return
	}
	// Your code here (2C).
	// Example:
	// r := bytes.NewBuffer(data)
	// d := labgob.NewDecoder(r)
	// var xxx
	// var yyy
	// if d.Decode(&xxx) != nil ||
	//    d.Decode(&yyy) != nil {
	//   error...
	// } else {
	//   rf.xxx = xxx
	//   rf.yyy = yyy
	// }
}

// the service says it has created a snapshot that has
// all info up to and including index. this means the
// service no longer needs the log through (and including)
// that index. Raft should now trim its log as much as possible.
func (rf *Raft) Snapshot(index int, snapshot []byte) {
	// Your code here (2D).

}

// RequestVoteArgs
// example RequestVote RPC arguments structure.
// field names must start with capital letters!
type RequestVoteArgs struct {
	// Your data here (2A, 2B).
	Term         int //	需要竞选的人的任期
	CandidateId  int // 需要竞选的人的Id
	LastLogIndex int // 竞选人日志条目最后索引
	LastLogTerm  int // 候选人最后日志条目的任期号
}

// RequestVoteReply
// example RequestVote RPC reply structure.
// field names must start with capital letters!
// 如果竞选者任期比自己的任期还短，那就不投票，返回false
// 如果当前节点的votedFor为空，且竞选者的日志条目跟收到者的一样新则把票投给该竞选者
type RequestVoteReply struct {
	// Your data here (2A).
	Term        int       // 投票方的term，如果竞选者比自己还低就改为这个
	VoteGranted bool      // 是否投票给了该竞选人
	VoteState   VoteState // 投票状态
}

// example RequestVote RPC handler.
// RequestVote
// example RequestVote RPC handler.
// 个人认为定时刷新的地方应该是别的节点与当前节点在数据上不冲突时就要刷新
// 因为如果不是数据冲突那么定时相当于防止自身去选举的一个心跳
// 如果是因为数据冲突，那么这个节点不用刷新定时是为了当前整个raft能尽快有个正确的leader
func (rf *Raft) RequestVote(args *RequestVoteArgs, reply *RequestVoteReply) {

	// Your code here (2A, 2B).

	defer fmt.Printf("[	    func-RequestVote-rf(%+v)		] : return %v\n", rf.me, *reply)

	rf.mu.Lock()
	defer rf.mu.Unlock()

	// 当前节点crash
	if rf.killed() {
		reply.VoteState = Killed
		reply.Term = -1
		reply.VoteGranted = false
		return
	}

	//reason: 出现网络分区，该竞选者已经OutOfDate(过时）
	//网络延迟 test中以节点 sleep后再次重连会导致这个问题
	if args.Term < rf.currentTerm {
		reply.VoteState = Expire
		reply.Term = rf.currentTerm
		reply.VoteGranted = false
		return
	}

	if args.Term > rf.currentTerm {
		// 重置自身的状态    将自身状态设置为follow 最近term 且在此term不投票
		rf.status = Follower
		rf.currentTerm = args.Term
		rf.votedFor = -1
	}

	fmt.Printf("[	    func-RequestVote-rf(%+v)		] : rf.voted: %v\n", rf.me, rf.votedFor)
	// 此时比自己任期小的都已经把票还原===>上面的argsterm>rfterm处理完成
	if rf.votedFor == -1 {

		currentLogIndex := len(rf.logs) - 1
		currentLogTerm := 0
		// 如果currentLogIndex下标不是-1就把term赋值过来
		// 2a不会进去 currentLogIndex 和term一直都是-1 和 0
		// 所以紧接着的两个log if是不会进入执行
		if currentLogIndex >= 0 {
			currentLogTerm = rf.logs[currentLogIndex].Term
		}

		//  If votedFor is null or candidateId, and candidate’s log is at least as up-to-date as receiver’s log, grant vote (§5.2, §5.4)
		// 论文里的第二个匹配条件，当前peer要符合arg两个参数的预期
		// 请求机的日志或者说版本落后响应机，正常来说这个log条目应该是同步的
		// 但是任期号大于等于响应机，所以说情况应该是sleep
		if args.LastLogIndex < currentLogIndex || args.LastLogTerm < currentLogTerm {
			reply.VoteState = Expire
			reply.VoteGranted = false
			reply.Term = rf.currentTerm
			return
		}

		// 给票数，并且返回true
		rf.votedFor = args.CandidateId

		reply.VoteState = Normal
		reply.Term = rf.currentTerm
		reply.VoteGranted = true

		rf.timer.Reset(rf.overtime)

		fmt.Printf("[	    func-RequestVote-rf(%v)		] : voted rf[%v]\n", rf.me, rf.votedFor)

	} else { // 只剩下任期相同，但是票已经给了，此时存在两种情况

		reply.VoteState = Voted
		reply.VoteGranted = false

		// 1、当前的节点是来自同一轮，不同竞选者的，但是票数已经给了(又或者它本身自己就是竞选者）
		if rf.votedFor != args.CandidateId {
			// 告诉reply票已经没了返回false
			return
		} else { // 2. 当前的节点票已经给了同一个人了，但是由于sleep等网络原因，又发送了一次请求
			// 重置自身状态
			rf.status = Follower
		}
		// 参选完重置自己的计时器
		rf.timer.Reset(rf.overtime)

	}
	return
}

// AppendEntriesArgs 由leader复制log条目，也可以当做是心跳连接，注释中的rf为leader节点
type AppendEntriesArgs struct {
	Term         int        // leader的任期
	LeaderId     int        // leader自身的ID
	PrevLogIndex int        // 预计要从哪里追加的index，因此每次要比当前的len(logs)多1 args初始化为：rf.nextIndex[i] - 1
	PrevLogTerm  int        // 追加新的日志的任期号(这边传的应该都是当前leader的任期号 args初始化为：rf.currentTerm
	Entries      []LogEntry // 预计存储的日志（为空时就是心跳连接）
	LeaderCommit int        // leader的commit index指的是最后一个被大多数机器都复制的日志Index
}

type AppendEntriesReply struct {
	Term     int                // leader的term可能是过时的，此时收到的Term用于更新他自己
	Success  bool               //	如果follower与Args中的PreLogIndex/PreLogTerm都匹配才会接过去新的日志（追加），不匹配直接返回false
	AppState AppendEntriesState // 追加状态
}

// 2a完成==> 但是这边没有对log进行append 只是单纯的去维持状态机的状态
func (rf *Raft) AppendEntries(args *AppendEntriesArgs, reply *AppendEntriesReply) {
	rf.mu.Lock()
	defer rf.mu.Unlock()

	// 节点crash
	if rf.killed() {
		reply.AppState = AppKilled
		reply.Term = -1
		reply.Success = false
		return
	}

	// 出现网络分区，args的任期，比当前raft的任期还小
	// 说明args之前所在的分区已经OutOfDate
	if args.Term < rf.currentTerm {
		reply.AppState = AppOutOfDate
		reply.Term = rf.currentTerm
		reply.Success = false
		return
	}

	// 对当前的rf进行ticker重置
	rf.currentTerm = args.Term
	rf.votedFor = args.LeaderId
	rf.status = Follower
	rf.timer.Reset(rf.overtime)

	// 对返回的reply进行赋值
	reply.AppState = AppNormal
	reply.Term = rf.currentTerm
	reply.Success = true
	return
}

// example code to send a RequestVote RPC to a server.
// server is the index of the target server in rf.peers[].
// expects RPC arguments in args.
// fills in *reply with RPC reply, so caller should
// pass &reply.
// the types of the args and reply passed to Call() must be
// the same as the types of the arguments declared in the
// handler function (including whether they are pointers).
//
// The labrpc package simulates a lossy network, in which servers
// may be unreachable, and in which requests and replies may be lost.
// Call() sends a request and waits for a reply. If a reply arrives
// within a timeout interval, Call() returns true; otherwise
// Call() returns false. Thus Call() may not return for a while.
// A false return can be caused by a dead server, a live server that
// can't be reached, a lost request, or a lost reply.
//
// Call() is guaranteed to return (perhaps after a delay) *except* if the
// handler function on the server side does not return.  Thus there
// is no need to implement your own timeouts around Call().
//
// look at the comments in ../labrpc/labrpc.go for more details.
//
// if you're having trouble getting RPC to work, check that you've
// capitalized all field names in structs passed over RPC, and
// that the caller passes the address of the reply struct with &, not
// the struct itself.
func (rf *Raft) sendRequestVote(server int, args *RequestVoteArgs, reply *RequestVoteReply, voteNums *int) bool {

	if rf.killed() {
		return false
	}
	ok := rf.peers[server].Call("Raft.RequestVote", args, reply)
	for !ok {
		// 失败重传
		if rf.killed() {
			return false
		}
		ok = rf.peers[server].Call("Raft.RequestVote", args, reply)

	}

	rf.mu.Lock()
	defer rf.mu.Unlock()
	fmt.Printf("[	sendRequestVote(%v) ] : send a election to %v\n", rf.me, server)
	// 由于网络分区，请求投票的人的term的比自己的还小，不给予投票
	if args.Term < rf.currentTerm {
		return false
	}

	// 对reply的返回情况进行分支处理
	switch reply.VoteState {
	// 消息过期有两种情况:
	// 1.是本身的term过期了比节点的还小
	// 2.是节点日志的条目落后于节点了
	case Expire:
		{
			rf.status = Follower
			rf.timer.Reset(rf.overtime)
			if reply.Term > rf.currentTerm {
				rf.currentTerm = reply.Term
				rf.votedFor = -1
			}
		}
	case Normal, Voted:
		//根据是否同意投票，收集选票数量
		if reply.VoteGranted && reply.Term == rf.currentTerm && *voteNums <= (len(rf.peers)/2) {
			fmt.Printf("[	sendRequestVote-func-rf(%v)		] recv vote form %v\n", rf.me, server)
			*voteNums++
		}

		// 票数超过一半
		if *voteNums >= (len(rf.peers)/2)+1 {

			*voteNums = 0
			// 本身就是leader在网络分区中更具有话语权的leader
			if rf.status == Leader {
				return ok
			}

			// 本身不是leader，那么需要初始化nextIndex数组
			rf.status = Leader
			rf.nextIndex = make([]int, len(rf.peers))
			for i, _ := range rf.nextIndex {
				rf.nextIndex[i] = len(rf.logs) + 1
			}
			rf.timer.Reset(HeartBeatTimeout)
			fmt.Printf("[	sendRequestVote-func-rf(%v)		] be a leader\n", rf.me)
		}
	case Killed:
		return false
	}
	return ok
}

func (rf *Raft) sendAppendEntries(server int, args *AppendEntriesArgs, reply *AppendEntriesReply, appendNums *int) bool {

	if rf.killed() {
		return false
	}

	// paper中5.3节第一段末尾提到，如果append失败应该不断的retries ,直到这个log成功的被store
	// 这也保证了日志一定会被写入，达到最终一致性
	ok := rf.peers[server].Call("Raft.AppendEntries", args, reply)
	for !ok {

		if rf.killed() {
			return false
		}
		ok = rf.peers[server].Call("Raft.AppendEntries", args, reply)

	}

	// ====>锁的位置:必须在加在这里否则加载前面retry时进入时，RPC也需要一个锁，但是又获取不到，因为锁已经被加上了
	// 每个raft节点只有一把锁,控制好锁,否则会出现死锁或者线程的值泄露
	// 因为这边是协程启动,上面rpc中调用别的节点方法如果加锁，而别的节点也正rpc调用，就获取不到别的raft节点的锁
	rf.mu.Lock()
	defer rf.mu.Unlock()

	// 对reply的返回状态进行分支
	switch reply.AppState {

	// 目标节点crash
	case AppKilled:
		{
			return false
		}

	// 目标节点正常返回
	case AppNormal:
		{
			// 2A的test目的是让Leader能不能连续任期，所以2A只需要对节点初始化然后返回就好
			return true
		}

	//If AppendEntries RPC received from new leader: convert to follower(paper - 5.2)
	//reason: 出现网络分区，该Leader已经OutOfDate(过时）
	case AppOutOfDate:

		// 该节点变成追随者,并重置rf状态
		rf.status = Follower
		rf.votedFor = -1
		rf.timer.Reset(rf.overtime) // 重置状态
		rf.currentTerm = reply.Term

	}
	return ok
}

// the service using Raft (e.g. a k/v server) wants to start
// agreement on the next command to be appended to Raft's log. if this
// server isn't the leader, returns false. otherwise start the
// agreement and return immediately. there is no guarantee that this
// command will ever be committed to the Raft log, since the leader
// may fail or lose an election. even if the Raft instance has been killed,
// this function should return gracefully.
//
// the first return value is the index that the command will appear at
// if it's ever committed. the second return value is the current
// term. the third return value is true if this server believes it is
// the leader.
func (rf *Raft) Start(command interface{}) (int, int, bool) {
	index := -1
	term := -1
	isLeader := true

	// Your code here (2B).

	return index, term, isLeader
}

// the tester doesn't halt goroutines created by Raft after each test,
// but it does call the Kill() method. your code can use killed() to
// check whether Kill() has been called. the use of atomic avoids the
// need for a lock.
//
// the issue is that long-running goroutines use memory and may chew
// up CPU time, perhaps causing later tests to fail and generating
// confusing debug output. any goroutine with a long-running loop
// should call killed() to check whether it should stop.
func (rf *Raft) Kill() {
	atomic.StoreInt32(&rf.dead, 1)
	// Your code here, if desired.
	rf.mu.Lock()
	rf.timer.Stop() //将自身的定时器停止防止进行新的选举
	rf.mu.Unlock()
}

func (rf *Raft) killed() bool {
	z := atomic.LoadInt32(&rf.dead)
	return z == 1
}

// raft的ticker(心脏)
// The ticker go routine starts a new election if this peer hasn't received
// heartsbeats recently.
func (rf *Raft) ticker() {

	for rf.killed() == false {
		//原本实现是检测到当前状态

		// Your code here to check if a leader election should
		// be started and to randomize sleeping time using

		// 当定时器结束进行超时选举
		select {

		case <-rf.timer.C:
			if rf.killed() {
				return
			}
			rf.mu.Lock() // 大锁,更细粒度的锁很难把控
			// 根据自身的status进行一次ticker
			switch rf.status {

			// follower变成竞选者
			case Follower:
				rf.status = Candidate
				fallthrough
			case Candidate:

				// 初始化自身的任期、并把票投给自己
				rf.currentTerm += 1
				rf.votedFor = rf.me
				votedNums := 1 // 统计自身的票数

				// 每轮选举开始时，重新设置选举超时
				rf.overtime = time.Duration(150+rand.Intn(200)) * time.Millisecond // 随机产生200-400ms
				rf.timer.Reset(rf.overtime)

				// 对自身以外的节点进行拉票
				for i := 0; i < len(rf.peers); i++ {
					if i == rf.me {
						continue
					}

					voteArgs := RequestVoteArgs{
						Term:         rf.currentTerm,
						CandidateId:  rf.me,
						LastLogIndex: len(rf.logs) - 1, // 最新的日志位置
						LastLogTerm:  0,
					}
					if len(rf.logs) > 0 { // 2a不会进去，因为2a没有对log操作
						voteArgs.LastLogTerm = rf.logs[len(rf.logs)-1].Term // 更新最新请求的term
					}

					voteReply := RequestVoteReply{}

					go rf.sendRequestVote(i, &voteArgs, &voteReply, &votedNums)
				}
			case Leader:
				// 进行心跳/日志同步
				appendNums := 1 // 对于正确返回的节点数量
				rf.timer.Reset(HeartBeatTimeout)
				// 构造msg
				for i := 0; i < len(rf.peers); i++ {
					if i == rf.me {
						continue
					}
					appendEntriesArgs := AppendEntriesArgs{
						Term:         rf.currentTerm,
						LeaderId:     rf.me,
						PrevLogIndex: 0, // 初始化为nextlogindex
						PrevLogTerm:  0, // 初始化为currentterm
						Entries:      nil,
						LeaderCommit: rf.commitIndex,
					}

					appendEntriesReply := AppendEntriesReply{}
					fmt.Printf("[	ticker(%v) ] : send a log to %v\n", rf.me, i)
					go rf.sendAppendEntries(i, &appendEntriesArgs, &appendEntriesReply, &appendNums)
				}
			}

			rf.mu.Unlock()
		}

	}
}

// the service or tester wants to create a Raft server. the ports
// of all the Raft servers (including this one) are in peers[]. this
// server's port is peers[me]. all the servers' peers[] arrays
// have the same order. persister is a place for this server to
// save its persistent state, and also initially holds the most
// recent saved state, if any. applyCh is a channel on which the
// tester or service expects Raft to send ApplyMsg messages.
// Make() must return quickly, so it should start goroutines
// for any long-running work.
// Make
// the service or tester wants to create a Raft server. the ports
// of all the Raft servers (including this one) are in peers[]. this
// server's port is peers[me]. all the servers' peers[] arrays
// have the same order. persister is a place for this server to
// save its persistent state, and also initially holds the most
// recent saved state, if any. applyCh is a channel on which the
// tester or service expects Raft to send ApplyMsg messages.
// Make() must return quickly, so it should start goroutines
// for any long-running work.
func Make(peers []*labrpc.ClientEnd, me int,
	persister *Persister, applyCh chan ApplyMsg) *Raft {
	rf := &Raft{}
	rf.peers = peers
	rf.persister = persister
	rf.me = me

	// Your initialization code here (2A, 2B, 2C).

	// 对应论文中的初始化
	rf.applyChan = applyCh //2B

	rf.currentTerm = 0
	rf.votedFor = -1
	rf.logs = make([]LogEntry, 0)

	rf.commitIndex = 0
	rf.lastApplied = 0

	rf.nextIndex = make([]int, len(peers))
	rf.matchIndex = make([]int, len(peers))

	rf.status = Follower
	rf.overtime = time.Duration(150+rand.Intn(200)) * time.Millisecond // 随机产生150-350ms
	rf.timer = time.NewTicker(rf.overtime)

	// initialize from state persisted before a crash
	rf.readPersist(persister.ReadRaftState())

	//fmt.Printf("[ 	Make-func-rf(%v) 	]:  %v\n", rf.me, rf.overtime)
	// start ticker goroutine to start elections
	go rf.ticker()

	return rf
}

// func (rf *Raft) setHeartBeat() {
// 	go func() {

// 		rf.HeartBeat <- true
// 	}()
// }
// func (rf *Raft) setWinElect() {
// 	go func() {

//			rf.WinElect <- true
//		}()
//	}
// func (rf *Raft) setVoted() {
// 	go func() {
// 		rf.Voted <- true
// 	}()
// }

// func (rf *Raft) updateState(state int32) {
// 	if rf.State == state {
// 		return
// 	}
// 	old_state := rf.State
// 	switch state {
// 	case STATE_FOLLWER:
// 		rf.State = STATE_FOLLWER
// 		rf.VoteFor = -1
// 	case STATE_CANDIDATE:
// 		rf.State = STATE_CANDIDATE
// 	case STATE_LEADER:
// 		rf.State = STATE_LEADER
// 		rf.boardCastAppendEntries()
// 	default:
// 		log.Printf("unknowned state %d \n", state)
// 	}
// 	log.Printf("In term %d machine %d update state from %d to %d \n", rf.CurrentTerm, rf.me, old_state, rf.State)
// }

// func (rf *Raft) startElection() {

// 	// 任期加一
// 	rf.mu.Lock()
// 	rf.VoteFor = rf.me
// 	rf.CntVoted = 1
// 	rf.CurrentTerm += 1
// 	args := RequestVoteArgs{
// 		Term:        rf.CurrentTerm,
// 		CandidateId: rf.me,
// 	}
// 	rf.mu.Unlock()
// 	for i := range rf.peers {
// 		if int(i) == rf.me {
// 			continue
// 		}
// 		go func(server int) {
// 			reply := RequestVoteReply{}

// 			log.Printf("in term %d machine %d send req vote to machine %d", rf.CurrentTerm, rf.me, server)
// 			if rf.State == STATE_CANDIDATE && rf.sendRequestVote(server, &args, &reply) {
// 				rf.mu.Lock()
// 				defer rf.mu.Unlock()
// 				if args.Term != rf.CurrentTerm {
// 					return
// 				}
// 				if reply.VoteGranted {
// 					rf.CntVoted += 1
// 					log.Printf("in term %d machine %d recv vote from machine %d", rf.CurrentTerm, rf.me, server)
// 					if rf.CntVoted > len(rf.peers)/2 {
// 						log.Println("win elect", rf.me)
// 						rf.setWinElect()
// 					}
// 				} else {
// 					if reply.Term > rf.CurrentTerm {
// 						rf.CurrentTerm = reply.Term
// 						rf.updateState(STATE_FOLLWER)
// 					}
// 				}
// 			} else {
// 				log.Printf("send request vote form %d to %d error!\n", rf.me, server)
// 			}

// 		}(int(i))
// 	}
// }

// func (rf *Raft) boardCastAppendEntries() {

// 	AppendEntriesFunc := func(server int) {
// 		reply := AppendEntriesReply{}

// 		args := AppendEntriesArgs{Term: rf.CurrentTerm, LeaderId: rf.me}
// 		if rf.State != STATE_LEADER {
// 			return
// 		}
// 		if rf.sendAppendEntries(server, &args, &reply) {
// 			rf.mu.Lock()
// 			defer rf.mu.Unlock()
// 			if rf.State != STATE_LEADER || rf.CurrentTerm < reply.Term || rf.CurrentTerm != args.Term {
// 				return
// 			}
// 			if rf.CurrentTerm > reply.Term {
// 				rf.CurrentTerm = reply.Term
// 				rf.updateState(STATE_FOLLWER)
// 			} else {
// 				return
// 			}
// 		}
// 		return
// 	}

// 	for i := range rf.peers {
// 		if int(i) == rf.me {
// 			continue
// 		}

// 		go func(server int) {

// 			AppendEntriesFunc(server)

// 		}(int(i))
// 	}
// }

// // timeout set
// func randTime() time.Duration {
// 	r := rand.New(rand.NewSource(time.Now().UnixMicro()))
// 	return time.Microsecond * time.Duration((r.Intn(150) + 400))
// }
