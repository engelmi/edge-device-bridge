package main

import (
	"fmt"

	"github.com/engelmi/edge-device-bridge/pkg/bluechi"
)

func main() {

	bluechiClient, err := bluechi.NewBlueChiClient()
	if err != nil {
		panic(err)
	}

	t, _ := bluechiClient.LastTimeSeen("/org/eclipse/bluechi/node/pi")
	fmt.Println(t)

	// monitor.WithUnitSignalHandler(bluechi.SignalUnitStateChanged, func(i []interface{}) {
	// 	fmt.Println("here i am")
	// })
	// monitor.Start(context.Background())
}
