package kafka

import (
	l "log"
	"os"
	"time"

	cluster "github.com/bsm/sarama-cluster"
	"github.com/juju/errors"
	"github.com/zssky/log"

	"github.com/Shopify/sarama"
	"github.com/dearcode/watcher/config"
	"github.com/dearcode/watcher/harvester"
	"github.com/dearcode/watcher/meta"
)

var (
	kh = kafkaHarvester{}
)

type kafkaHarvester struct {
	consumer *cluster.Consumer
	msgChan  chan<- *meta.Message
}

func init() {
	harvester.Register("kafka", &kh)
}

func (kh *kafkaHarvester) Init(hc config.HarvesterConfig, msgChan chan<- *meta.Message) error {
	cc := cluster.NewConfig()
	cc.ClientID = hc.ClientID
	cc.Consumer.Offsets.Initial = sarama.OffsetOldest
	cc.Consumer.MaxWaitTime = time.Second
	cc.Consumer.Return.Errors = true
	cc.Group.Return.Notifications = true

	sarama.Logger = l.New(os.Stdout, "", l.Lshortfile|l.LstdFlags)

	consumer, err := cluster.NewConsumer(hc.Brokers, hc.Group, hc.Topics, cc)
	if err != nil {
		return errors.Annotatef(err, "NewConsumer:%+v", hc)
	}

	kh.consumer = consumer
	kh.msgChan = msgChan

	go kh.run()

	return nil
}

func (kh *kafkaHarvester) Stop() {
	kh.consumer.Close()
}

func (kh *kafkaHarvester) run() {
	for {
		select {
		case msg, ok := <-kh.consumer.Messages():
			if !ok {
				log.Errorf("consumer Messages error")
				return
			}
			kh.msgChan <- meta.NewMessage(msg.Topic, string(msg.Value))
			log.Debugf("topic:%v, offset:%v, value:%v", msg.Topic, msg.Offset, string(msg.Value))
			kh.consumer.MarkOffset(msg, "")
		case err, ok := <-kh.consumer.Errors():
			log.Errorf("consumer Error:%v, ok:%v", err, ok)
			return
		case ntf, ok := <-kh.consumer.Notifications():
			if ok {
				log.Infof("Rebalanced: %#v", ntf)
			}
		}
	}
}
