package consumerLibrary

import (
	"github.com/aliyun/aliyun-log-go-sdk"
	"time"
)

type ShardConsumerWorker struct{
	*ConsumerClient
	*ConsumerCheckPointTracker
	ConsumerShutDownFlag bool
	LastFetchLogGroup *sls.LogGroupList
	NextFetchCursor  string
	LastFetchGroupCount int
	LastFetchtime   int64
	ConsumerStatus 	string
	Process 		func(a int, logGroup *sls.LogGroupList)
	ShardId			int
}


func InitShardConsumerWorker(shardId int,consumerClient *ConsumerClient,do func(a int, logGroup *sls.LogGroupList))*ShardConsumerWorker{
	shardConsumeWorker := &ShardConsumerWorker{
		ConsumerShutDownFlag:false,
		Process:do,
		ConsumerCheckPointTracker:InitConsumerCheckpointTracker(shardId,consumerClient),
		ConsumerClient:consumerClient,
		ConsumerStatus:INITIALIZ,
		ShardId:shardId,
		LastFetchtime:0,
	}
	return shardConsumeWorker
}

func (consumer *ShardConsumerWorker)consume(){
	a := make(chan int)
	b := make(chan int)
	c := make(chan int)
	d := make(chan int)
	if consumer.ConsumerShutDownFlag == true{
		consumer.ConsumerStatus = SHUTTING_DOWN
	}
	if consumer.ConsumerStatus == SHUTTING_DOWN  {
		go func(){
			d <-4
		}()
	}
	if consumer.ConsumerStatus == INITIALIZ {
		go func(){
			a <- 1
		}()
	}
	if consumer.ConsumerStatus == PROCESS && consumer.LastFetchLogGroup == nil{
		Info.Println("给拉日志发信号了")
		go func(){
			b <- 2
		}()
	}
	if consumer.ConsumerStatus == PROCESS && consumer.LastFetchLogGroup != nil{
		Info.Println("给消费日志发信号了")
		go func(){
			c <- 3
		}()
	}
	select{
	case _,ok:=<-a:
		if ok{
			consumer.NextFetchCursor = consumer.ConsumerInitializeTask()
			consumer.ConsumerStatus = PROCESS
		}
	case _,ok:= <-b:
		if ok{
			Info.Println("执行过拉的动作")
			var is_generate_fetch_task  = true
			if consumer.LastFetchGroupCount < 100 {
				is_generate_fetch_task = (time.Now().UnixNano()/1e6 - consumer.LastFetchtime) > 500    //转换成500毫秒
			}
			if consumer.LastFetchGroupCount < 500 {
				is_generate_fetch_task = (time.Now().UnixNano()/1e6 - consumer.LastFetchtime) > 200
			}
			if consumer.LastFetchGroupCount < 1000 {
				is_generate_fetch_task = (time.Now().UnixNano()/1e6 - consumer.LastFetchtime) > 50
			}
			if is_generate_fetch_task {
				consumer.LastFetchtime = time.Now().UnixNano()/1e6
				consumer.LastFetchLogGroup, consumer.NextFetchCursor = consumer.ConsumerFetchTask()
				consumer.SetMemoryCheckPoint(consumer.NextFetchCursor)
				consumer.LastFetchGroupCount = GetLogCount(consumer.LastFetchLogGroup)
				if consumer.LastFetchGroupCount == 0{
					consumer.LastFetchLogGroup = nil
				}
				Info.Printf("shard %v get log conut : %v",consumer.ShardId,consumer.LastFetchGroupCount)
			}

		}
	case _,ok:=<-c:
		if ok{
			Info.Println("执行过消费的动作")
			consumer.ConsumerProcessTask()
			consumer.LastFetchLogGroup = nil
			// consumer.LastFetchGroupCount = 0 // 这应该不用给0
		}
	case _,ok:= <-d:
		if ok{
			// 强制刷新当前的检查点
			consumer.MflushCheckPoint()
			consumer.ConsumerStatus = SHUTDOWN_COMPLETE
			Info.Printf("shardworker %v are shut down complete",consumer.ShardId)
		}
	}

}

func (consumer *ShardConsumerWorker)ConsumerShutDown(){
	consumer.ConsumerShutDownFlag = true
	if !consumer.IsShutDown(){
		consumer.consume()
	}
}

func (consumer *ShardConsumerWorker)IsShutDown()bool{
	return consumer.ConsumerStatus == SHUTDOWN_COMPLETE
}