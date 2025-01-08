package kafka

import (
	"github.com/IBM/sarama"
	"github.com/pkg/errors"
)

func CreateTopic(brokers []string, topicName string) error {
	config := sarama.NewConfig()
	config.Version = sarama.V2_0_1_0
	admin, err := sarama.NewClusterAdmin(brokers, config)
	if err != nil {
		return errors.Wrap(err, "Error while creating cluster admin")
	}
	defer func() { _ = admin.Close() }()
	err = admin.CreateTopic(topicName, &sarama.TopicDetail{
		NumPartitions:     1,
		ReplicationFactor: 1,
	}, false)
	if err != nil {
		return errors.Wrap(err, "Error while creating topic: ")
	}

	return nil
}
