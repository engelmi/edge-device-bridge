package mqtt

import (
	"fmt"

	pahomqtt "github.com/eclipse/paho.mqtt.golang"
)

type MQTTClient struct {
	pahoClient pahomqtt.Client

	ClientID string
}

func NewMQTTClient(
	clientID string,
	broker string,
	port uint16,
	user string,
	password string,
) (*MQTTClient, error) {

	mqttClient := &MQTTClient{
		pahoClient: nil,
		ClientID:   clientID,
	}

	opts := pahomqtt.NewClientOptions()
	opts.AddBroker(fmt.Sprintf("tcp://%s:%d", broker, port))
	opts.SetClientID(clientID)
	opts.SetUsername(user)
	opts.SetPassword(password)
	opts.SetDefaultPublishHandler(mqttClient.defaultMessageHandler)
	opts.OnConnect = mqttClient.onConnectHandler
	opts.OnConnectionLost = mqttClient.onDisconnectHandler
	client := pahomqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		return nil, token.Error()
	}
	mqttClient.pahoClient = client

	return mqttClient, nil
}

func (c *MQTTClient) defaultMessageHandler(client pahomqtt.Client, msg pahomqtt.Message) {
	fmt.Printf("Received message: %s from topic: %s\n", msg.Payload(), msg.Topic())
}

func (c *MQTTClient) onConnectHandler(client pahomqtt.Client) {
	fmt.Println("Connected")
}

func (c *MQTTClient) onDisconnectHandler(client pahomqtt.Client, err error) {
	fmt.Printf("Connection lost: %v", err)
}

type MessageHandler func(*MQTTClient, []byte)

func (c *MQTTClient) Subscribe(topic string, handler MessageHandler) {
	c.pahoClient.Subscribe(topic, 0, func(_ pahomqtt.Client, m pahomqtt.Message) {
		handler(c, m.Payload())
	})
}

func (c *MQTTClient) Publish(topic string, b []byte) error {
	if token := c.pahoClient.Publish(topic, 0, false, b); token.Wait() && token.Error() != nil {
		return token.Error()
	}
	return nil
}

func (c *MQTTClient) Close() {
	if c.pahoClient != nil {
		c.pahoClient.Disconnect(0)
	}
}
