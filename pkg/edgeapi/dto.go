package edgeapi

import (
	"encoding/json"
)

type RegisterRequest struct {
	DeviceID   string `json:"device-id,omitempty"`
	DeviceType string `json:"device-type,omitempty"`
}

type RegisterResponse struct {
	Result string `json:"result,omitempty"`
}

type Workload struct {
	Name     string `json:"name,omitempty"`
	State    string `json:"state,omitempty"`
	SubState string `json:"substate,omitempty"`
}

type Node struct {
	Name              string     `json:"name,omitempty"`
	Status            string     `json:"status,omitempty"`
	LastSeenTimestamp string     `json:"lastSeenTimestamp,omitempty"`
	Workloads         []Workload `json:"workloads,omitempty"`
}

type DeviceUpdateRequest struct {
	ID    string `json:"id,omitempty"`
	Nodes []Node `json:"nodes,omitempty"`
}

func Unmarshal[T any](data []byte) (*T, error) {
	out := new(T)
	if err := json.Unmarshal(data, out); err != nil {
		return nil, err
	}
	return out, nil
}

func Marshal(i interface{}) ([]byte, error) {
	return json.Marshal(i)
}
