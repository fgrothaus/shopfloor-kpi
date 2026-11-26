package mqtt_broker

import (
	"encoding/json"
	"fmt"
	"log"

	MQTT "github.com/eclipse/paho.mqtt.golang"
	"github.com/fgrothaus/shopfloor-kpi/internal/model"
)

func Connect(broker *string, id *string) MQTT.Client {
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
