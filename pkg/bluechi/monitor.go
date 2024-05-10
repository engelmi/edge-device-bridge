package bluechi

import (
	"context"
	"fmt"
	"strings"

	"github.com/godbus/dbus/v5"
)

type SignalFunc func([]interface{}, string, string)

type Monitor struct {
	conn *dbus.Conn

	monitorPath   string
	monitorObject dbus.BusObject

	handlers   map[string]SignalFunc
	signalChan chan *dbus.Signal
}

func NewMonitor(client *BlueChiClient) (*Monitor, error) {
	m := &Monitor{
		conn:       client.dbusConn,
		handlers:   map[string]SignalFunc{},
		signalChan: make(chan *dbus.Signal, 10),
	}

	monitorPath := ""
	busObject := m.conn.Object(DBusBlueChiInterface, ObjectPathBlueChi)
	err := busObject.Call(MethodControllerCreateMonitor, 0).Store(&monitorPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create monitor: %v", err)
	}

	m.monitorPath = monitorPath
	m.monitorObject = m.conn.Object(DBusBlueChiInterface, dbus.ObjectPath(monitorPath))

	return m, nil
}

func (m *Monitor) WithUnitSignalHandler(s SignalType, handler SignalFunc) error {
	if err := m.monitorObject.AddMatchSignal(MonitorInterface, string(s)).Err; err != nil {
		return fmt.Errorf("failed to add handler for signal '%s': %v", string(s), err)
	}
	m.handlers[fmt.Sprintf("%s/%s", MonitorInterface, string(s))] = handler
	return nil
}

func (m *Monitor) WithNodeSignalHandler(handler SignalFunc) error {
	key := fmt.Sprintf("%s/node/", ObjectPathBlueChi)
	if _, ok := m.handlers[key]; ok {
		return fmt.Errorf("already handler for node signals addded")
	}

	busObject := m.conn.Object(DBusBlueChiInterface, ObjectPathBlueChi)
	err := busObject.AddMatchSignal(FreedesktopDBusProps, FreedesktopDBusPropsChanged).Err
	if err != nil {
		return fmt.Errorf("failed to add handler for node signal handler: %v", err)
	}
	m.handlers[key] = handler
	return nil
}

func (m *Monitor) Start(ctx context.Context) error {
	var subscriptionID uint64
	err := m.monitorObject.Call(MethodMonitorSubscribe, 0, "*", "*").Store(&subscriptionID)
	if err != nil {
		return fmt.Errorf("failed to create subscription: %v", err)
	}

	propsChangedName := fmt.Sprintf("%s.%s", FreedesktopDBusProps, FreedesktopDBusPropsChanged)

	m.conn.Signal(m.signalChan)
	for v := range m.signalChan {
		matched := false
		for name, handler := range m.handlers {
			if
			// unit changed handler check
			(v.Name == name && v.Path == m.monitorObject.Path()) ||
				// node changed handler present
				(strings.HasPrefix(string(v.Path), name) && v.Name == propsChangedName) {
				handler(v.Body, v.Name, string(v.Path))
				matched = true
				break
			}
		}
		if !matched {
			fmt.Println("Unexpected signal: ", v.Name)
		}
	}
	return nil
}

func (m *Monitor) Close() {
	if m.signalChan != nil {
		close(m.signalChan)
	}
}
