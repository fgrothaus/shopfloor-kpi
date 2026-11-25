//go:build ignore

package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/gopcua/opcua"
	"github.com/gopcua/opcua/ua"
)

const (
	endpoint = "opc.tcp://localhost:4840"
	stateId  = "ns=2;s=Presse_01.State"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second) // 10 Sekunden Timeout für die Verbindung festlegen.
	defer cancel()

	client, err := opcua.NewClient(endpoint, opcua.SecurityMode(ua.MessageSecurityModeNone))

	// nil ist ein Fehlerobjekt. Wenn err = nil heißt, kein Fehler. err != nil heißt Fehler wurde gesetzt.
	if err != nil {
		log.Fatalf("Client anlegen: %v", err)
	}

	if err := client.Connect(ctx); err != nil {
		log.Fatalf("verbinden mit %s: %v", endpoint, err)
	}
	defer client.Close(ctx)
	fmt.Println("Verbunden mit", endpoint)

	//********************* Knoten abfragen *********************
	id, err := ua.ParseNodeID(stateId) // ParseNodeID wandelt NodeID in NodeID-Objekt um
	if err != nil {
		log.Fatalf("NodeID parsen: %v", err)
	}

	// &ua.ReadRequest gibt einen Pointer auf ein ReadRequest-Struct zurück aus dem UA-Package.
	// In ua.ReadRequest werden die Knoten, die gelesen werden sollen, und weitere Parameter angegeben/vorbereitet für client.Read.
	req := &ua.ReadRequest{
		MaxAge: 2000,
		NodesToRead: []*ua.ReadValueID{
			{
				NodeID: id,
			},
		},
		TimestampsToReturn: ua.TimestampsToReturnBoth,
	}

	// client.Read sendet die ReadRequest an den Server und gibt eine ReadResponse zurück.
	resp, err := client.Read(ctx, req)
	if err != nil {
		log.Fatalf("Knoten lesen: %v", err)
	}

	// resp.Results ist ein Slice von Pointern auf ReadResponse-Structs. Es können mehrere Knoten gleichzeitig gelesen werden in ReadRequest.
	result := resp.Results[0]
	if result.Status != ua.StatusOK {
		log.Fatalf("Knoten lesen: %v", result.Status)
	}

	fmt.Printf("State = %v (Typ %T)\n", result.Value.Value(), result.Value.Value()) // In Go ist die Schreibweise von Werten relevant. Value.value heißt nicht exportiert und nur im Package nutzbar. Deswegen muss Value.Value() aufgerufen werden (der Getter für value)
	fmt.Println("Status:", result.Status)
	fmt.Println("SourceTimestamp:", result.SourceTimestamp)
	fmt.Println("ServerTimestamp:", result.ServerTimestamp)

}
