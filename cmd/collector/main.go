package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"

	"github.com/fgrothaus/shopfloor-kpi/internal/model"
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

	fmt.Println("Collector läuft. Beenden mit Strg+C.")
	for m := range measurements {
		fmt.Printf("%+v\n", m)
	}
	fmt.Println("Collector beendet.")
}
