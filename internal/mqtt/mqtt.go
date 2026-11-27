package mqtt_broker

import (
	"encoding/json"
	"fmt"
	"log"

	MQTT "github.com/eclipse/paho.mqtt.golang"
	"github.com/fgrothaus/shopfloor-kpi/internal/model"
)

func PubConnect(broker *string, id *string) MQTT.Client {
	opts := MQTT.NewClientOptions()
	opts.AddBroker(*broker)
	opts.SetClientID(*id)

	client := MQTT.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		panic(token.Error())
	}
	fmt.Println("Sample Publisher Started")

	return client
}

func Publish(client MQTT.Client, topic *string, qos *int, measurement model.Measurement) {

	bytes, err := json.Marshal(measurement)
	if err != nil {
		log.Fatalf("Fehler beim Serialisieren der Nachricht: %v", err)
	}

	token := client.Publish(*topic, byte(*qos), false, bytes)
	token.Wait()
	if err := token.Error(); err != nil {
		log.Fatalf("Fehler beim Publishen: %v", err)
	}
}

func SubConnect(broker *string, id *string) (MQTT.Client, <-chan [2]string) {
	choke := make(chan [2]string, 100)

	opts := MQTT.NewClientOptions()
	opts.AddBroker(*broker)
	opts.SetClientID(*id)
	opts.SetOrderMatters(false)
	opts.SetDefaultPublishHandler(func(client MQTT.Client, msg MQTT.Message) {
		choke <- [2]string{msg.Topic(), string(msg.Payload())}
	})

	client := MQTT.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		panic(token.Error())
	}
	fmt.Println("Sample Publisher Started")

	return client, choke
}

func Subscribe(client MQTT.Client, topic *string, qos byte) error {

	token := client.Subscribe(*topic, qos, nil)
	token.Wait()
	return token.Error()

}
