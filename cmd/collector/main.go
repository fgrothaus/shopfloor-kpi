package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"

	"github.com/fgrothaus/shopfloor-kpi/internal/model"
	mqtt_broker "github.com/fgrothaus/shopfloor-kpi/internal/mqtt"
	"github.com/fgrothaus/shopfloor-kpi/internal/opcua"
)

const endpoint = "opc.tcp://localhost:4840"
const machine = "Presse_01"

var nodes = []string{
	"ns=2;s=Presse_01.State",
	"ns=2;s=Presse_01.ActivePower",
	"ns=2;s=Presse_01.AlarmCode",
	"ns=2;s=Presse_01.EnergyTotal",
	"ns=2;s=Presse_01.GoodParts",
	"ns=2;s=Presse_01.ScrapParts",
	"ns=2;s=Presse_01.OilTemperature",
	"ns=2;s=Presse_01.OrderNumber",
	"ns=2;s=Presse_01.PressForce",
	"ns=2;s=Presse_01.StrokeCounter",
	"ns=2;s=Presse_01.TargetCycleTime",
	"ns=2;s=Presse_01.TargetQuantity",
}

func main() {

	topic := flag.String("topic", "presse_01/measurement", "The topic name to/from which to publish/subscribe")
	broker := flag.String("broker", "tcp://localhost:1883", "The broker URI. ex: tcp://10.10.1.1:1883")
	id := flag.String("id", "shopfloor-collector", "The ClientID (optional)")
	qos := flag.Int("qos", 0, "The Quality of Service 0,1,2 (default 0)")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop() // defer ist wie ein finally. Wird immer ausgeführt. Es wird am Ende ausgeführt, aber an den Anfang geschrieben,
	// damit man sofort sieht, auf was sich das defer bezieht. Es wird in umgekehrter Reihenfolge ausgeführt, also das letzte defer wird zuerst ausgeführt (LIFO).

	// Channel erzeugen, der model.Measurements über den Channel transportiert
	measurements := make(chan model.Measurement)

	// OPC UA läuft in einer eigenen Goroutine und liefert über den Channel
	go func() {
		defer close(measurements)
		if err := opcua.Run(ctx, endpoint, machine, nodes, measurements); err != nil {
			log.Printf("opcua: %v", err)
		}
	}()

	mqttClient := mqtt_broker.PubConnect(broker, id)
	defer mqttClient.Disconnect(250)

	fmt.Println("Collector läuft. Beenden mit Strg+C.")
	for m := range measurements {
		mqtt_broker.Publish(mqttClient, topic, qos, m)
		fmt.Printf("%+v\n", m)
	}
	fmt.Println("Collector beendet.")
}
