package kafka

import (
	"fmt"
	"strconv"
	"time"

	"github.com/IBM/sarama"
)

type Producer struct {
	AsyncProducer sarama.AsyncProducer
	SyncProducer  sarama.SyncProducer
	config        *sarama.Config
	topic         string
	enqueued      int
	errors        int
}

// NewSyncProducer 创建producer
// brokers: kafka brokers 格式 []string{"10.122.7.8:9092", "10.122.7.15:9092", "10.122.7.3:9092"}
// topics: kafka topics 格式 []string{"topicA","topicB","topicC"}
// enableSuccessesMessage: true 发送消息之后Product函数会返回successMessage *sarama.ProducerMessage
func NewSyncProducer(brokers []string, topic string, enableSuccessesMessage bool, isSyncProducer bool) (producer *Producer, err error) {
	config := sarama.NewConfig()
	config.Producer.Retry.Max = 5
	config.Version = sarama.V2_0_1_0
	config.Producer.RequiredAcks = sarama.WaitForAll // 发送完数据需要leader和follow都确认
	config.Producer.Retry.Max = 3                    // 设置重试3次
	config.Producer.Retry.Backoff = 500 * time.Millisecond
	config.Producer.Return.Successes = enableSuccessesMessage // 同步模式下必须为true

	producer = &Producer{
		topic:  topic,
		config: config,
	}

	if isSyncProducer == false {
		producer.AsyncProducer, err = sarama.NewAsyncProducer(brokers, config)
		if err != nil {
			fmt.Errorf("error: %v", err)
			return
		}
	} else {
		producer.SyncProducer, err = sarama.NewSyncProducer(brokers, config)
		if err != nil {
			fmt.Errorf("error: %v", err)
			return
		}
	}
	return
}

// AsyncSendMessage 发送异步消息给kafka
// message 消息内容
// successMessage enableSuccessesMessage为true是,会有消息返回
func (p *Producer) AsyncSendMessage(message []byte) (successMessage *sarama.ProducerMessage, err error) {
	strTime := strconv.Itoa(int(time.Now().Unix()))
	msg := &sarama.ProducerMessage{
		Topic: p.topic,
		Key:   sarama.StringEncoder(strTime),
		Value: sarama.StringEncoder(message),
	}

	p.AsyncProducer.Input() <- msg
	p.enqueued++

	// wait response
	select {
	case successMessage = <-p.AsyncProducer.Successes():
	case err = <-p.AsyncProducer.Errors():
		if err != nil {
			p.errors++
			err = fmt.Errorf("failed to message, error: %v", err)
		}
	default:
	}

	return
}

// SyncSendMessage 发送同步消息给kafka
func (p *Producer) SyncSendMessage(message []byte) (partition int32, offset int64, err error) {
	strTime := strconv.Itoa(int(time.Now().Unix()))

	msg := &sarama.ProducerMessage{
		Topic: p.topic,
		Key:   sarama.StringEncoder(strTime),
		Value: sarama.StringEncoder(message),
	}

	partition, offset, err = p.SyncProducer.SendMessage(msg)
	if err != nil {
		p.errors++
		err = fmt.Errorf("failed to message, error: %v", err)
	}
	p.enqueued++
	return
}
