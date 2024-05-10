package main

import (
	"context"
	"fmt"
	"time"

	"github.com/engelmi/edge-device-bridge/pkg"
	"github.com/engelmi/edge-device-bridge/pkg/bluechi"
	"github.com/engelmi/edge-device-bridge/pkg/mqtt"
	"github.com/google/uuid"
)

func main() {
	clientID := uuid.New()

	mqttClient, err := mqtt.NewMQTTClient(clientID.String(), "localhost", 8883, "random", "pass")
	if err != nil {
		fmt.Printf("Failed to create MQTT client: %v\n", err)
		return
	}
	defer mqttClient.Close()

	bluechiClient, err := bluechi.NewBlueChiClient()
	if err != nil {
		fmt.Printf("Failed to create BlueChi client: %v\n", err)
		return
	}

	bridge, err := pkg.NewEdgeBridge(mqttClient, bluechiClient, 5*time.Second)
	if err != nil {
		fmt.Printf("Failed to create MQTT client: %v\n", err)
		return
	}

	bridge.Start(context.Background())
}
