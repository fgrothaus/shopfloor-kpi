package opcua

import (
	"context"
	"fmt"
	"log"
	"time"

	gopcua "github.com/gopcua/opcua"
	"github.com/gopcua/opcua/ua"

	"github.com/fgrothaus/shopfloor-kpi/internal/model"
)

// Run verbindet sich mit dem Server, abonniert die Knoten und schreibt jede
// Wertänderung als Measurement in den Channel out.
// Run läuft, bis ctx beendet wird oder ein Fehler auftritt.
func Run(ctx context.Context, endpoint, machine string, nodes []string, out chan<- model.Measurement) error {
	client, err := gopcua.NewClient(
		endpoint,
		gopcua.SecurityMode(ua.MessageSecurityModeNone),
		gopcua.DialTimeout(10*time.Second),
	)
	if err != nil {
		return fmt.Errorf("client anlegen: %w", err)
	}

	if err := client.Connect(ctx); err != nil {
		return fmt.Errorf("verbinden mit %s: %w", endpoint, err)
	}
	defer client.Close(context.Background())
	log.Println("Verbunden mit", endpoint)

	//********************* Subscription anlegen *********************
	notifyCh := make(chan *gopcua.PublishNotificationData) // Channel für Server-Notifications anlegen

	sub, err := client.Subscribe(
		ctx,
		&gopcua.SubscriptionParameters{Interval: 500 * time.Millisecond},
		notifyCh,
	)
	if err != nil {
		return fmt.Errorf("subscription anlegen: %w", err)
	}
	defer sub.Cancel(context.Background())

	//********************* Monitored Items anlegen *********************
	names := make(map[uint32]string) // ClientHandle → NodeId
	var requests []*ua.MonitoredItemCreateRequest

	for i, s := range nodes {
		id, err := ua.ParseNodeID(s)
		if err != nil {
			return fmt.Errorf("nodeid parsen %s: %w", s, err)
		}
		handle := uint32(i + 1)
		names[handle] = s
		requests = append(requests,
			gopcua.NewMonitoredItemCreateRequestWithDefaults(id, ua.AttributeIDValue, handle))
	}

	res, err := sub.Monitor(ctx, ua.TimestampsToReturnBoth, requests...)
	if err != nil {
		return fmt.Errorf("monitored items anlegen: %w", err)
	}

	for i, r := range res.Results {
		if r.StatusCode != ua.StatusOK {
			log.Printf("%s abgelehnt: %v", nodes[i], r.StatusCode)
		}
	}
	log.Printf("%d Knoten werden überwacht", len(nodes))

	//********************* Notifications empfangen *********************
	for {
		select {
		case <-ctx.Done():
			return nil

		case msg := <-notifyCh:
			if msg.Error != nil {
				log.Printf("fehler in der subscription: %v", msg.Error)
				continue
			}

			switch x := msg.Value.(type) {
			case *ua.DataChangeNotification:
				for _, item := range x.MonitoredItems {
					dv := item.Value
					var value any
					if dv.Value != nil {
						value = dv.Value.Value()
					}

					out <- model.Measurement{
						Machine:   machine,
						Signal:    names[item.ClientHandle],
						Value:     value,
						Good:      dv.Status == ua.StatusOK,
						Timestamp: dv.SourceTimestamp,
					}

					// // - = Linksbündig, 32/20 = Breite an Zeichen
					// fmt.Printf("%-32s = %-20v | %v | %s\n",
					// 	names[item.ClientHandle],
					// 	value,
					// 	dv.Status,
					// 	dv.SourceTimestamp.Format("15:04:05.000"),
					// )
				}
			}
		}
	}
}
