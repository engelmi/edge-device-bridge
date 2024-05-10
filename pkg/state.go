package pkg

type SystemdService struct {
	Name     string
	State    string
	SubState string
}

type Node struct {
	Name              string
	Status            string
	LastSeenTimestamp string
	Services          map[string]SystemdService
}

type BlueChiState struct {
	Nodes map[string]Node
}

func (b BlueChiState) IsEmpty() bool {
	return len(b.Nodes) == 0
}
