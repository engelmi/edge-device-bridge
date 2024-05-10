package bluechi

import (
	"fmt"
	"time"

	"github.com/godbus/dbus/v5"
)

type SignalType string

const (
	DBusBlueChiInterface = "org.eclipse.bluechi"
	ObjectPathBlueChi    = "/org/eclipse/bluechi"

	MethodControllerListNodes     = "org.eclipse.bluechi.Controller.ListNodes"
	MethodControllerListUnits     = "org.eclipse.bluechi.Controller.ListUnits"
	MethodControllerCreateMonitor = "org.eclipse.bluechi.Controller.CreateMonitor"

	DBusBlueChiNodeInterface    = "org.eclipse.bluechi.Node"
	ObjectPathNodeBase          = "/org/eclipse/bluechi/node/"
	ObjectPathNodeTemplate      = ObjectPathNodeBase + "%s"
	MethodNodeListUnits         = "org.eclipse.bluechi.Node.ListUnits"
	MethodNodeLastSeenTimestamp = "org.eclipse.bluechi.Node.LastSeenTimestamp"

	MonitorInterface       = "org.eclipse.bluechi.Monitor"
	MethodMonitorSubscribe = "org.eclipse.bluechi.Monitor.Subscribe"

	FreedesktopDBusProps        = "org.freedesktop.DBus.Properties"
	FreedesktopDBusPropsChanged = "PropertiesChanged"

	SignalUnitNew               SignalType = "UnitNew"
	SignalUnitRemoved           SignalType = "UnitRemoved"
	SignalUnitPropertiesChanged SignalType = "UnitPropertiesChanged"
	SignalUnitStateChanged      SignalType = "UnitStateChanged"
)

type BlueChiClient struct {
	dbusConn *dbus.Conn
}

func NewBlueChiClient() (*BlueChiClient, error) {
	conn, err := dbus.ConnectSystemBus()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to system bus: %v", err)
	}

	return &BlueChiClient{
		dbusConn: conn,
	}, nil
}

func (c *BlueChiClient) ListNodes() ([][]interface{}, error) {
	var nodes [][]interface{}
	busObject := c.dbusConn.Object(DBusBlueChiInterface, ObjectPathBlueChi)
	err := busObject.Call(MethodControllerListNodes, 0).Store(&nodes)
	if err != nil {
		return nil, fmt.Errorf("failed to list all units: %v", err)
	}

	return nodes, nil
}

func (c *BlueChiClient) ListUnitsOn(node string) ([][]interface{}, error) {
	var units [][]interface{}
	busObject := c.dbusConn.Object(DBusBlueChiInterface, dbus.ObjectPath(fmt.Sprintf(ObjectPathNodeTemplate, node)))
	err := busObject.Call(MethodNodeListUnits, 0).Store(&units)
	if err != nil {
		return nil, fmt.Errorf("failed to list all units on node '%s': %v", node, err)
	}

	return units, nil
}

func (c *BlueChiClient) LastTimeSeen(nodePath dbus.ObjectPath) (time.Time, error) {
	nodeObject := c.dbusConn.Object(DBusBlueChiInterface, nodePath)
	prop, err := nodeObject.GetProperty(MethodNodeLastSeenTimestamp)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to get last seen timestamp for node path '%s': %v", nodePath, err)
	}
	return time.Unix(int64(prop.Value().(uint64)), 0), nil
}

func (c *BlueChiClient) Close() {
	if c.dbusConn != nil {
		c.dbusConn.Close()
	}
}
