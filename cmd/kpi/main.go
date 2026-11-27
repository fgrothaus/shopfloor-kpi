package main

import (
	"flag"
	"log"

	mqtt_broker "github.com/fgrothaus/shopfloor-kpi/internal/mqtt"
)

// main ist die Main-Goroutine und blockiert den Prozess, wenn der messageChannel leer ist.
func main() {

	topic := flag.String("topic", "presse_01/measurement", "The topic name to/from which to publish/subscribe")
	broker := flag.String("broker", "tcp://localhost:1883", "The broker URI. ex: tcp://10.10.1.1:1883")
	id := flag.String("id", "shopfloor-kpi", "The ClientID (optional)")
	qos := flag.Int("qos", 0, "The Quality of Service 0,1,2 (default 0)")
	flag.Parse()

	client, messageChannel := mqtt_broker.SubConnect(broker, id)
	defer client.Disconnect(250)

	if err := mqtt_broker.Subscribe(client, topic, byte(*qos)); err != nil {
		log.Fatalf("subscribe: %v", err)
	}

	for msg := range messageChannel {
		// Prüfen, was für Daten es sind, sprich das Feld Signal auswerten "Signal":"ns=2;s=Presse_01.ActivePower"
		// Je nach Messwert KPI berechnen.
		log.Printf("Received message on topic %s: %s", msg[0], msg[1])
	}

}
