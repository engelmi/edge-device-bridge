package pkg

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/engelmi/edge-device-bridge/pkg/bluechi"
	"github.com/engelmi/edge-device-bridge/pkg/edgeapi"
	"github.com/engelmi/edge-device-bridge/pkg/mqtt"
	"github.com/godbus/dbus/v5"
)

const TopicBase = "redhat/edge/device/"
const TopicRegister = TopicBase + "register"

type EdgeBridge struct {
	systemID       string
	systemType     string
	mqttClient     *mqtt.MQTTClient
	bluechiClient  *bluechi.BlueChiClient
	bluechiMonitor *bluechi.Monitor

	blueChiState BlueChiState

	topicBaseDevice   string
	topicDeviceUpdate string

	minUpdateInterval time.Duration
	lastUpdateSent    time.Time
}

func NewEdgeBridge(mqttClient *mqtt.MQTTClient,
	bluechiClient *bluechi.BlueChiClient,
	minUpdateInterval time.Duration,
) (*EdgeBridge, error) {
	monitor, err := bluechi.NewMonitor(bluechiClient)
	if err != nil {
		return nil, err
	}

	topicBaseDevice := fmt.Sprintf("%s/%s", TopicBase, mqttClient.ClientID)
	topicDeviceUpdate := fmt.Sprintf("%s/update", topicBaseDevice)

	return &EdgeBridge{
		systemID:   mqttClient.ClientID,
		systemType: "VeryEdgy",

		mqttClient:     mqttClient,
		bluechiClient:  bluechiClient,
		bluechiMonitor: monitor,

		blueChiState: BlueChiState{
			Nodes: map[string]Node{},
		},

		topicBaseDevice:   topicBaseDevice,
		topicDeviceUpdate: topicDeviceUpdate,

		minUpdateInterval: minUpdateInterval,
		lastUpdateSent:    time.Time{},
	}, nil
}

func (bridge *EdgeBridge) setupBlueChiListener() {
	// bridge.bluechiMonitor.WithSignalHandler(bluechi.SignalUnitNew, nil)
	// bridge.bluechiMonitor.WithSignalHandler(bluechi.SignalUnitRemoved, nil)
	// bridge.bluechiMonitor.WithSignalHandler(bluechi.SignalUnitPropertiesChanged, nil)
	bridge.bluechiMonitor.WithUnitSignalHandler(bluechi.SignalUnitStateChanged, bridge.updateStateUnitStateChanged)
	bridge.bluechiMonitor.WithNodeSignalHandler(bridge.updateStateNodeChanged)
}

func (bridge *EdgeBridge) updateStateNodeChanged(data []interface{}, name string, path string) {
	ifaceName := data[0].(string)
	if ifaceName != bluechi.DBusBlueChiNodeInterface {
		fmt.Printf(
			"Received property changed signal for '%s', but '%s' expected. Skipping...\n",
			ifaceName,
			bluechi.DBusBlueChiNodeInterface,
		)
		return
	}

	changedValues, ok := data[1].(map[string]dbus.Variant)
	if !ok {
		fmt.Println("Received invalid property changed signal")
		return
	}
	if val, ok := changedValues["Status"]; ok {
		nodeName := strings.Replace(path, bluechi.ObjectPathNodeBase, "", 1)

		if _, ok := bridge.blueChiState.Nodes[nodeName]; !ok {
			bridge.blueChiState.Nodes[nodeName] = Node{}
		}
		node := bridge.blueChiState.Nodes[nodeName]
		node.Name = nodeName
		node.Status = strings.Trim(val.String(), "\"")
		if node.Status == "online" {
			node.LastSeenTimestamp = "now"
			units, err := bridge.bluechiClient.ListUnitsOn(nodeName)
			if err != nil {
				fmt.Printf("Failed to list units on '%s': %v\n", nodeName, err)
				return
			}

			node.Services = map[string]SystemdService{}
			for _, unit := range units {
				unitName := unit[0].(string)
				node.Services[unitName] = SystemdService{
					Name:     unitName,
					State:    unit[3].(string),
					SubState: unit[4].(string),
				}
			}
		} else {
			node.LastSeenTimestamp = time.Now().Format(time.RFC3339)
		}
		bridge.blueChiState.Nodes[nodeName] = node

		// Publish right away
		fmt.Printf("Published state update node %s, %s\n", nodeName, node.Status)
		bridge.publishBlueChiStateUpdate()
	}
}

func (bridge *EdgeBridge) updateStateUnitStateChanged(data []interface{}, name string, path string) {
	nodeName := data[0].(string)
	unitName := data[1].(string)
	activeState := data[2].(string)
	subState := data[3].(string)

	if _, ok := bridge.blueChiState.Nodes[nodeName]; !ok {
		bridge.blueChiState.Nodes[nodeName] = Node{
			Name:              nodeName,
			Status:            "online",
			LastSeenTimestamp: "now",
			Services:          map[string]SystemdService{},
		}
	}

	if _, ok := bridge.blueChiState.Nodes[nodeName].Services[unitName]; !ok {
		bridge.blueChiState.Nodes[nodeName].Services[unitName] = SystemdService{
			Name: unitName,
		}
	}
	service := bridge.blueChiState.Nodes[nodeName].Services[unitName]
	service.State = activeState
	service.SubState = subState
	bridge.blueChiState.Nodes[nodeName].Services[unitName] = service

	if time.Since(bridge.lastUpdateSent) > bridge.minUpdateInterval {
		if err := bridge.publishBlueChiStateUpdate(); err != nil {
			fmt.Println(err)
		}
	}
}

func (bridge *EdgeBridge) initBlueChiState() error {
	nodes, err := bridge.bluechiClient.ListNodes()
	if err != nil {
		return err
	}

	for _, listedNode := range nodes {
		nodeName := listedNode[0].(string)
		status := listedNode[2].(string)

		node := Node{
			Name:     nodeName,
			Status:   status,
			Services: map[string]SystemdService{},
		}

		if status == "offline" {
			lastSeenTimestamp, err := bridge.bluechiClient.LastTimeSeen(listedNode[1].(dbus.ObjectPath))
			if err != nil {
				node.LastSeenTimestamp = "never"
			} else {
				node.LastSeenTimestamp = lastSeenTimestamp.Format(time.RFC3339)
			}
		} else {
			node.LastSeenTimestamp = "now"
			units, err := bridge.bluechiClient.ListUnitsOn(nodeName)
			if err != nil {
				return err
			}

			for _, unit := range units {
				unitName := unit[0].(string)
				node.Services[unitName] = SystemdService{
					Name:     unitName,
					State:    unit[3].(string),
					SubState: unit[4].(string),
				}
			}
		}

		bridge.blueChiState.Nodes[nodeName] = node
	}

	return nil
}

func (bridge *EdgeBridge) startBlueChiStatePusher(_ context.Context) {
	go func(b *EdgeBridge) {
		for {
			time.Sleep(b.minUpdateInterval)
			if !b.blueChiState.IsEmpty() {
				fmt.Println("BlueChiState not empty, pushing changes...")
				b.publishBlueChiStateUpdate()
			}
		}
	}(bridge)
}

func (bridge *EdgeBridge) Start(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	deviceTopic := fmt.Sprintf("%s/%s", TopicBase, bridge.systemID)
	deviceTopicRegister := fmt.Sprintf("%s/register", deviceTopic)
	bridge.mqttClient.Subscribe(deviceTopicRegister, func(m *mqtt.MQTTClient, b []byte) {
		fmt.Println("Successfully registered")
		resp, err := edgeapi.Unmarshal[edgeapi.RegisterResponse](b)
		if err != nil || resp.Result != "success" {
			fmt.Printf("Register failed: Result: %s, Unmarshal error: %v\n", resp.Result, err)
			cancel()
			return
		}
		if err := bridge.onRegisterSuccess(); err != nil {
			fmt.Println(err)
			cancel()
		}
	})

	b, err := edgeapi.Marshal(edgeapi.RegisterRequest{
		DeviceID:   bridge.systemID,
		DeviceType: bridge.systemType,
	})
	if err != nil {
		return fmt.Errorf("failed to marshal register request: %v", err)
	}
	if err := bridge.mqttClient.Publish(TopicRegister, b); err != nil {
		return fmt.Errorf("failed to publish register request: %v", err)
	}

	<-ctx.Done()
	return nil
}

func (bridge *EdgeBridge) onRegisterSuccess() error {
	bridge.setupBlueChiListener()
	if err := bridge.initBlueChiState(); err != nil {
		return err
	}
	if err := bridge.publishBlueChiStateUpdate(); err != nil {
		return err
	}

	go bridge.bluechiMonitor.Start(context.Background())
	go bridge.startBlueChiStatePusher(context.Background())

	return nil
}

func (bridge *EdgeBridge) publishBlueChiStateUpdate() error {

	updateRequest := edgeapi.DeviceUpdateRequest{
		ID:    bridge.systemID,
		Nodes: []edgeapi.Node{},
	}

	for _, blueChiNode := range bridge.blueChiState.Nodes {
		node := edgeapi.Node{
			Name:              blueChiNode.Name,
			Status:            blueChiNode.Status,
			LastSeenTimestamp: blueChiNode.LastSeenTimestamp,
			Workloads:         []edgeapi.Workload{},
		}

		for _, blueChiService := range blueChiNode.Services {
			node.Workloads = append(node.Workloads, edgeapi.Workload{
				Name:     blueChiService.Name,
				State:    blueChiService.State,
				SubState: blueChiService.SubState,
			})
		}

		updateRequest.Nodes = append(updateRequest.Nodes, node)
	}

	b, err := edgeapi.Marshal(updateRequest)
	if err != nil {
		return fmt.Errorf("failed marshalling update: %v", err)
	}
	fmt.Println()
	fmt.Println()
	fmt.Println(string(b))
	fmt.Println()
	fmt.Println()

	err = bridge.mqttClient.Publish(bridge.topicDeviceUpdate, b)
	if err != nil {
		return fmt.Errorf("failed to publish device update: %v", err)
	}

	// clear state to create "diff" updates
	bridge.blueChiState = BlueChiState{
		Nodes: map[string]Node{},
	}
	return nil
}
