package kafka

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/IBM/sarama"
)

type ConsumerMessage func(*sarama.ConsumerMessage) (err error)

type Consumer struct {
	ready           chan bool
	brokers         []string
	topics          []string
	GroupClient     sarama.ConsumerGroup
	Client          sarama.Consumer
	ctx             context.Context
	ConsumerMessage ConsumerMessage
}

// NewConsumerGroup 创建一个consumer group client
// 创建会话: Consume —> newSession —> newConsumerGroupSession —> handler.Setup
// 会话执行: Consume —> newSession —> newConsumerGroupSession —> consume —> s.handler.ConsumeClaim
// 会话结束: Consume —>release —> s.handler.Cleanup
// version 表示kafka cluster版本,测试过程中发现使用version会引发问题,暂时不生效
func NewConsumerGroup(version, groupID string, brokers, topics []string, ctx context.Context, isOldest bool, fn ConsumerMessage) (consumer *Consumer, err error) {
	// var kafkaVersion sarama.KafkaVersion
	// kafkaVersion, err = sarama.ParseKafkaVersion(version)
	// if err != nil {
	// 	fmt.Errorf("error parsing Kafka version: %v", err)
	// 	return
	// }
	config := sarama.NewConfig()
	if isOldest {
		config.Consumer.Offsets.Initial = sarama.OffsetOldest
	}
	// config.Version = kafkaVersion
	var client sarama.ConsumerGroup
	client, err = sarama.NewConsumerGroup(brokers, groupID, config)
	if err != nil {
		return
	}

	consumer = &Consumer{
		ready:           make(chan bool, 0),
		GroupClient:     client,
		brokers:         brokers,
		topics:          topics,
		ctx:             ctx,
		ConsumerMessage: fn,
	}
	return
}

// StartGroup 启动consumer进行消费
// 每个分区只能由同一个消费组内的一个consumer来消费;如果当前只有一个partition就不需要有多个消费者
func (c *Consumer) StartGroup(ctx context.Context, wg *sync.WaitGroup) {
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			if err := c.GroupClient.Consume(ctx, c.topics, c); err != nil {
				log.Fatalf("Error from consumer: %v", err)
			}
			// check if context was cancelled, signaling that the consumer should stop
			if ctx.Err() != nil {
				log.Println(ctx.Err())
				return
			}
			c.ready = make(chan bool)
		}
	}()
	<-c.ready
	log.Println("consumer up and running")
}

func (c *Consumer) Setup(session sarama.ConsumerGroupSession) error {
	fmt.Printf("%v", session.Claims())
	close(c.ready)
	return nil
}

func (c *Consumer) Cleanup(sarama.ConsumerGroupSession) error {
	return nil
}

func (c *Consumer) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) (err error) {
	for message := range claim.Messages() {
		err = c.ConsumerMessage(message)
		if err != nil {
			fmt.Errorf("consumeClaim deal message failed, error: %v", err)
			return
		}

		session.MarkMessage(message, "")
	}
	return
}

func NewConsumer(brokers, topics []string, ctx context.Context, fn ConsumerMessage) (consumer *Consumer, err error) {
	if len(topics) == 0 {
		err = fmt.Errorf("consumer topics is empty")
		return
	}

	config := sarama.NewConfig()
	var client sarama.Consumer
	client, err = sarama.NewConsumer(brokers, config)
	if err != nil {
		return
	}

	consumer = &Consumer{
		Client:          client,
		brokers:         brokers,
		topics:          topics,
		ctx:             ctx,
		ConsumerMessage: fn,
	}
	return
}

// StartConsumer 启动consumer
// 每个topic的partition只能有一个consumer进行消费,根据partition数量进行并发
func (c *Consumer) StartConsumer(wg *sync.WaitGroup) {
	for _, topic := range c.topics {
		// 根据topic取到所有的分区
		partitions, err := c.Client.Partitions(topic)
		if err != nil {
			fmt.Errorf("failed to get list of partition, err: %v", err)
			return
		}

		// 遍历所有的分区
		for _, partition := range partitions {
			// 针对每个分区创建一个对应的分区消费者
			go c.startPartitionConsumer(topic, partition, wg)
		}
	}
}

func (c *Consumer) startPartitionConsumer(topic string, partition int32, wg *sync.WaitGroup) {
	if wg != nil {
		wg.Add(1)
		defer wg.Done()
	}

	partitionConsumer, err := c.Client.ConsumePartition(topic, partition, sarama.OffsetOldest)
	if err != nil {
		fmt.Printf("failed to start consumer for partition %d,err:%v\n", partition, err)
		return
	}

	// 消费数据
	for {
		select {
		case msg := <-partitionConsumer.Messages():
			c.ConsumerMessage(msg)
		case <-c.ctx.Done():
			partitionConsumer.AsyncClose()
			break
		}
	}
}
